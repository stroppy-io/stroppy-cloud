package run

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The queue: a launch stores the run queued (pending, never handed to
// Graphene) and the queue starts it once the tenant has room for it — its
// pipelines pushed at this server's revision, a free max_concurrent_runs
// slot, a free cell slot of its suite, provider quota for its machines and,
// on Yandex, free blocks of the profile's address pool for its subnets (the
// queue allocates them at admission and stores them in the run's spec).
// A run that does not fit waits; the ones behind it that fit go first.
// Nothing waits forever by mistake: a run that can never fit (more than a
// quota's whole limit) is refused at launch.

// WithQueue gives the queue what it checks besides quotas.
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
				if !ok {
					continue
				}
				if d.specs[i] != nil {
					if err := s.repo.SetQueuedSpec(ctx, r.ID, d.specs[i]); err != nil {
						return err
					}
					r.RunSpec = d.specs[i]
				}
				admitted = append(admitted, r)
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
	// specs are the rewritten specs of admitted runs (their subnets).
	specs []json.RawMessage
}

// decide walks the queue in order: each run either takes its room (suite
// slot, quota) or records what it waits for and lets the next try.
//
//nolint:funlen,gocognit // one ordered checklist
func (s *Service) decide(ctx context.Context, tenantID uuid.UUID, queued []Run, quotas map[uuid.UUID]quotaRead) (decision, error) {
	d := decision{admit: make([]bool, len(queued)), waits: make([]Queue, len(queued)), specs: make([]json.RawMessage, len(queued))}
	// Another pass may have admitted everything between the read and the lock.
	if len(queued) == 0 {
		return d, nil
	}
	// Launch checked the author's rights; a run admitted later still must
	// not start for a tenant a platform admin suspended meanwhile.
	if suspended, err := s.access.Suspended(ctx, tenantID); err != nil {
		return d, err
	} else if suspended {
		for i := range queued {
			d.waits[i] = Queue{WaitingFor: WaitSuspended, Position: i + 1}
		}
		return d, nil
	}
	if s.pipelines != nil {
		if ready, status := s.pipelines.PipelinesReady(ctx, queued[0].GrapheneNamespace); !ready {
			for i := range queued {
				d.waits[i] = Queue{WaitingFor: WaitPipelines, Position: i + 1, Detail: status}
			}
			return d, nil
		}
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
	pools := map[uuid.UUID]*subnetPool{}
	for i, r := range queued {
		wait := Queue{Position: i + 1}
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
			case !qr.profile.Configured():
				// A profile from before the profile network: UpgradeStale
				// gives it one; no run starts on a network of its own.
				wait.WaitingFor, wait.Detail = WaitProvider, string(provider.StatusVerifying)
				d.waits[i] = wait
				continue
			}
			if sf, _ := quotaShortfall(qr.quotas, held[profileID], spec); sf != nil {
				wait.WaitingFor, wait.Quota, wait.Zone = WaitQuota, sf.quota.Name, sf.quota.Zone
				wait.Need, wait.Free, wait.Unit = sf.need, sf.free, sf.quota.Unit
				d.waits[i] = wait
				continue
			}
			if spec.Provider.Kind == pipelinespec.ProviderYandex {
				pool, ok := pools[profileID]
				if !ok {
					runs, err := s.repo.NetworkRuns(ctx, tenantID, profileID, time.Now().Add(-subnetCleanup))
					if err != nil {
						return d, err
					}
					pool = newSubnetPool(runs)
					pools[profileID] = pool
				}
				rewritten, waitPool, err := allocateRun(pool, spec, r.RunSpec)
				if err != nil {
					return d, err
				}
				if waitPool != "" {
					wait.WaitingFor, wait.Detail = WaitSubnets, waitPool
					d.waits[i] = wait
					continue
				}
				d.specs[i] = rewritten
				if rewritten != nil {
					_ = json.Unmarshal(rewritten, &spec) //nolint:errcheck // just marshaled
				}
			}
			held[profileID] = append(held[profileID], spec)
		}
		d.admit[i] = true
		if r.SuiteRunID != nil {
			cells[*r.SuiteRunID]++
		}
	}
	return d, nil
}

// allocateRun gives a Yandex run its subnets from the pool and returns the
// spec carrying them; waitPool names the pool when it has no room. A spec
// the zones cannot be read from passes unchanged: the pipeline refuses it
// with the reason instead of the run waiting forever.
func allocateRun(pool *subnetPool, spec pipelinespec.Run, raw json.RawMessage) (rewritten json.RawMessage, waitPool string, err error) {
	network, err := pipelinespec.YandexNetworkOf(spec.Provider.Settings)
	if err != nil {
		return nil, "", nil //nolint:nilerr // the pipeline reports it
	}
	zones, err := pipelinespec.YandexZones(spec)
	if err != nil {
		return nil, "", nil //nolint:nilerr // the pipeline reports it
	}
	subnets, err := pool.allocate(network.Pool(), zones)
	switch {
	case errors.Is(err, errPoolFull):
		return nil, network.Pool(), nil
	case err != nil:
		return nil, "", nil //nolint:nilerr // the pipeline reports it
	}
	// Only network.subnets changes: the stored spec is otherwise kept
	// byte for byte (it was baked at launch).
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "", err
	}
	var net map[string]json.RawMessage
	if len(doc["network"]) > 0 {
		if err := json.Unmarshal(doc["network"], &net); err != nil {
			return nil, "", err
		}
	}
	if net == nil {
		net = map[string]json.RawMessage{}
	}
	if net["subnets"], err = json.Marshal(subnets); err != nil {
		return nil, "", err
	}
	if doc["network"], err = json.Marshal(net); err != nil {
		return nil, "", err
	}
	rewritten, err = json.Marshal(doc)
	return rewritten, "", err
}

func (s *Service) warn(msg string, err error) {
	if s.log != nil {
		s.log.Warn(msg, xlog.Error("error", err))
	}
}
