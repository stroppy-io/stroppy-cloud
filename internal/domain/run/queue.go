package run

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The queue: a launch stores the run queued (pending, never handed to
// Graphene) and the queue starts it once the tenant has room for it — its
// pipelines pushed at this server's revision, a free max_concurrent_runs
// slot, a free cell slot of its suite, and provider quota for its machines.
// A run that does not fit waits; the ones behind it that fit go first.
// Nothing waits forever by mistake: a run that can never fit (more than a
// quota's whole limit) is refused at launch.

// WithQueue gives the queue what it checks besides limits and quotas.
func (s *Service) WithQueue(pipelines PipelineReadiness, suites SuiteConcurrency, log *xlog.Logger) {
	s.pipelines, s.suites, s.log = pipelines, suites, log
}

// Nudge asks the queue loop for a pass now (a run finished, a tenant got
// its pipelines). Never blocks.
func (s *Service) Nudge() {
	select {
	case s.nudge <- struct{}{}:
	default:
	}
}

// RunQueue admits queued runs on every tick and nudge until ctx ends.
func (s *Service) RunQueue(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		s.AdmitQueued(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-s.nudge:
		}
	}
}

// AdmitQueued makes one pass over every tenant with queued runs and starts
// the admitted ones; returns how many started.
func (s *Service) AdmitQueued(ctx context.Context) int {
	queued, err := s.repo.Queued(ctx)
	if err != nil {
		s.warn("queue: list", err)
		return 0
	}
	byTenant := map[uuid.UUID][]Run{}
	var order []uuid.UUID
	for _, r := range queued {
		if _, ok := byTenant[r.TenantID]; !ok {
			order = append(order, r.TenantID)
		}
		byTenant[r.TenantID] = append(byTenant[r.TenantID], r)
	}
	started := 0
	for _, tenantID := range order {
		started += s.admitTenant(ctx, tenantID, byTenant[tenantID])
	}
	return started
}

// admitTenantNow is the launch path: one pass over the tenant right after
// its run was stored, so a run with room starts without waiting a tick.
func (s *Service) admitTenantNow(ctx context.Context, tenantID uuid.UUID) {
	queued, err := s.repo.QueuedOf(ctx, tenantID)
	if err != nil {
		s.warn("queue: list", err)
		return
	}
	s.admitTenant(ctx, tenantID, queued)
}

// admitTenant decides the tenant's queued runs in order under the tenant
// lock (one decision at a time across servers), then starts the admitted.
func (s *Service) admitTenant(ctx context.Context, tenantID uuid.UUID, queued []Run) int {
	if len(queued) == 0 {
		return 0
	}
	// Quotas are read before the lock: a refresh runs a pipeline.
	quotas := map[uuid.UUID]quotaRead{}
	for _, r := range queued {
		id := r.Snapshot.ProviderProfile.ID
		if _, ok := quotas[id]; ok {
			continue
		}
		quotas[id] = s.readQuotas(ctx, tenantID, id)
	}
	var admitted []Run
	err := s.repo.Admit(ctx, tenantID, func(ctx context.Context) error {
		admitted = admitted[:0]
		// The list read before the lock may be stale (another pass admitted
		// some, a launch added some): decide on the one under the lock.
		var err error
		if queued, err = s.repo.QueuedOf(ctx, tenantID); err != nil {
			return err
		}
		for _, r := range queued {
			id := r.Snapshot.ProviderProfile.ID
			if _, ok := quotas[id]; !ok {
				quotas[id] = s.readQuotas(ctx, tenantID, id)
			}
		}
		d, err := s.decide(ctx, tenantID, queued, quotas)
		if err != nil {
			return err
		}
		for i, r := range queued {
			if d.admit[i] {
				ok, err := s.repo.AdmitQueued(ctx, r.ID)
				if err != nil {
					return err
				}
				if ok {
					admitted = append(admitted, r)
				}
				continue
			}
			if d.waits[i] != r.Queue {
				if err := s.repo.SetQueue(ctx, r.ID, d.waits[i]); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		s.warn("queue: admit", err)
		return 0
	}
	for _, r := range admitted {
		now := time.Now().UTC()
		r.AdmittedAt = &now
		if _, err := s.Start(ctx, r); err != nil {
			s.warn("queue: start", err)
		}
	}
	return len(admitted)
}

type quotaRead struct {
	profile provider.Profile
	quotas  []provider.Quota
	err     error
}

func (s *Service) readQuotas(ctx context.Context, tenantID, profileID uuid.UUID) quotaRead {
	p, err := s.profiles.ByID(ctx, tenantID, profileID)
	if err != nil {
		return quotaRead{err: err}
	}
	if p.Status != provider.StatusReady {
		return quotaRead{profile: p}
	}
	q, err := s.profiles.FreshQuotas(ctx, p)
	return quotaRead{profile: p, quotas: q, err: err}
}

// decision is the verdict per queued run, by index.
type decision struct {
	admit []bool
	waits []Queue
}

// decide walks the queue in order: each run either takes its room (slot,
// suite slot, quota) or records what it waits for and lets the next try.
//
//nolint:funlen,gocognit // one ordered checklist
func (s *Service) decide(ctx context.Context, tenantID uuid.UUID, queued []Run, quotas map[uuid.UUID]quotaRead) (decision, error) {
	d := decision{admit: make([]bool, len(queued)), waits: make([]Queue, len(queued))}
	if s.pipelines != nil {
		if ready, status := s.pipelines.PipelinesReady(ctx, queued[0].GrapheneNamespace); !ready {
			for i := range queued {
				d.waits[i] = Queue{WaitingFor: WaitPipelines, Position: i + 1, Detail: status}
			}
			return d, nil
		}
	}
	lim, err := s.limits.EffectiveLimits(ctx, tenantID)
	if err != nil {
		return d, err
	}
	live, err := s.repo.LiveCount(ctx, tenantID)
	if err != nil {
		return d, err
	}
	cells, err := s.repo.LiveCells(ctx, tenantID)
	if err != nil {
		return d, err
	}
	starting, err := s.repo.Starting(ctx, tenantID)
	if err != nil {
		return d, err
	}
	held := map[uuid.UUID][]pipelinespec.Run{}
	for _, st := range starting {
		var spec pipelinespec.Run
		if json.Unmarshal(st.Spec, &spec) == nil && spec.Resume == nil {
			held[st.ProfileID] = append(held[st.ProfileID], spec)
		}
	}
	concurrency := map[uuid.UUID]int{}
	for i, r := range queued {
		wait := Queue{Position: i + 1}
		if lim.MaxConcurrentRuns > 0 && live >= lim.MaxConcurrentRuns {
			wait.WaitingFor, wait.Live, wait.Limit = WaitSlot, live, lim.MaxConcurrentRuns
			d.waits[i] = wait
			continue
		}
		if r.SuiteRunID != nil && s.suites != nil {
			sr := *r.SuiteRunID
			c, ok := concurrency[sr]
			if !ok {
				if c, err = s.suites.SuiteConcurrency(ctx, sr); err != nil {
					return d, err
				}
				concurrency[sr] = c
			}
			if c > 0 && cells[sr] >= c {
				wait.WaitingFor, wait.Live, wait.Limit = WaitSuiteSlot, cells[sr], c
				d.waits[i] = wait
				continue
			}
		}
		var spec pipelinespec.Run
		if err := json.Unmarshal(r.RunSpec, &spec); err != nil {
			return d, err
		}
		profileID := r.Snapshot.ProviderProfile.ID
		if spec.Resume == nil {
			qr := quotas[profileID]
			switch {
			case qr.err != nil:
				wait.WaitingFor, wait.Detail = WaitQuotaUnknown, qr.err.Error()
				d.waits[i] = wait
				continue
			case qr.profile.Status != provider.StatusReady:
				wait.WaitingFor, wait.Detail = WaitProvider, string(qr.profile.Status)
				d.waits[i] = wait
				continue
			}
			if sf, _ := quotaShortfall(qr.quotas, held[profileID], spec); sf != nil {
				wait.WaitingFor, wait.Quota, wait.Zone = WaitQuota, sf.quota.Name, sf.quota.Zone
				wait.Need, wait.Free, wait.Unit = sf.need, sf.free, sf.quota.Unit
				d.waits[i] = wait
				continue
			}
			held[profileID] = append(held[profileID], spec)
		}
		d.admit[i] = true
		live++
		if r.SuiteRunID != nil {
			cells[*r.SuiteRunID]++
		}
	}
	return d, nil
}

func (s *Service) warn(msg string, err error) {
	if s.log != nil {
		s.log.Warn(msg, xlog.Error("error", err))
	}
}
