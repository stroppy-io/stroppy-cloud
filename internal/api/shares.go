package api

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/go-faster/jx"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/compare"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/share"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// sharesOf lists the active shares of a target (run.shares).
func (h *Handler) sharesOf(kind share.Kind, id uuid.UUID) []oas.Ref {
	out := []oas.Ref{}
	if h.deps.Shares == nil {
		return out
	}
	for _, r := range h.deps.Shares.OfTarget(context.Background(), kind, id) {
		out = append(out, oas.Ref{ID: r.ID, Name: oas.NewOptString(r.Name)})
	}
	return out
}

func (h *Handler) shareOf(s share.Share) *oas.Share {
	u, _ := url.Parse(h.deps.Shares.URL(s)) //nolint:errcheck // built from config + token
	out := &oas.Share{
		ID: s.ID, Token: s.Token, Scope: oas.ShareScope(s.Scope), Active: s.Active(), ExpiresAt: optTime(s.ExpiresAt), RevokedAt: optTime(s.RevokedAt),
		CapturedAt: oas.NewOptDateTime(s.CapturedAt), ViewCount: oas.NewOptInt(s.ViewCount), CreatedAt: s.CreatedAt,
		Target: oas.ShareTarget{Kind: oas.ShareTargetKind(s.Kind), RunIds: s.RunIDs},
	}
	if u != nil {
		out.URL = *u
	}
	if out.Target.RunIds == nil {
		out.Target.RunIds = []uuid.UUID{}
	}
	if s.TargetID != nil {
		out.Target.ID = *s.TargetID
	}
	if s.TargetName != "" {
		out.Target.Name = oas.NewOptString(s.TargetName)
	}
	if s.Title != "" {
		out.Title = oas.NewOptString(s.Title)
	}
	if s.CreatedBy != nil {
		out.CreatedBy = oas.NewOptUserRef(oas.UserRef{ID: s.CreatedBy.String()})
	}
	return out
}

func shareCreateFrom(req *oas.ShareCreate) (share.Create, error) {
	in := share.Create{Title: req.Title.Or(""), Scope: share.Scope(req.Scope.Or(oas.ShareScopeOverview))}
	if v, ok := req.TTL.Get(); ok && v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return in, errs.Invalid("ttl is not a positive duration")
		}
		in.TTL = &d
	}
	return in, nil
}

// ShareRun — publish a run.
func (h *Handler) ShareRun(ctx context.Context, req *oas.ShareCreate, params oas.ShareRunParams) (*oas.Share, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	in, err := shareCreateFrom(req)
	if err != nil {
		return nil, err
	}
	id := params.ID
	in.Kind, in.TargetID = share.KindRun, &id
	s, err := h.deps.Shares.Create(ctx, a, t.ID, in)
	if err != nil {
		return nil, err
	}
	return h.shareOf(s), nil
}

// ShareSuiteRun — publish a suite run.
func (h *Handler) ShareSuiteRun(ctx context.Context, req *oas.ShareCreate, params oas.ShareSuiteRunParams) (*oas.Share, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	in, err := shareCreateFrom(req)
	if err != nil {
		return nil, err
	}
	id := params.ID
	in.Kind, in.TargetID = share.KindSuiteRun, &id
	s, err := h.deps.Shares.Create(ctx, a, t.ID, in)
	if err != nil {
		return nil, err
	}
	return h.shareOf(s), nil
}

func compareRequestOf(runIDs []uuid.UUID, baseline oas.OptUUID, keys []string, deadband oas.OptFloat64) compare.Request {
	req := compare.Request{RunIDs: runIDs, MetricKeys: keys, DeadbandPct: deadband.Or(0)}
	if b, ok := baseline.Get(); ok {
		req.BaselineID = &b
	}
	return req
}

// ShareComparison — publish a comparison.
func (h *Handler) ShareComparison(ctx context.Context, req *oas.ShareComparisonReq, params oas.ShareComparisonParams) (*oas.Share, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	in, err := shareCreateFrom(&oas.ShareCreate{TTL: req.TTL, Scope: req.Scope, Title: req.Title})
	if err != nil {
		return nil, err
	}
	cr := compareRequestOf(req.RunIds, req.BaselineRunID, req.MetricKeys, req.DeadbandPct)
	in.Kind, in.Comparison = share.KindComparison, &cr
	s, err := h.deps.Shares.Create(ctx, a, t.ID, in)
	if err != nil {
		return nil, err
	}
	return h.shareOf(s), nil
}

// ListShares — of the tenant.
func (h *Handler) ListShares(ctx context.Context, params oas.ListSharesParams) (*oas.ListSharesOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	offset, err := cursorOffset(params.Cursor)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := share.ListQuery{Limit: limit + 1, Offset: offset}
	if v, ok := params.TargetKind.Get(); ok {
		q.Kind = string(v)
	}
	if v, ok := params.TargetID.Get(); ok {
		q.TargetID = v.String()
	}
	if v, ok := params.Active.Get(); ok {
		q.Active = &v
	}
	list, err := h.deps.Shares.List(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.ListSharesOK{Data: make([]oas.Share, 0, len(list)), Meta: meta}
	for _, s := range list {
		out.Data = append(out.Data, *h.shareOf(s))
	}
	return out, nil
}

// GetShare — one.
func (h *Handler) GetShare(ctx context.Context, params oas.GetShareParams) (*oas.Share, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	s, err := h.deps.Shares.Get(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.shareOf(s), nil
}

// PatchShare — ttl / scope.
func (h *Handler) PatchShare(ctx context.Context, req *oas.PatchShareReq, params oas.PatchShareParams) (*oas.Share, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p := share.Patch{}
	if v, ok := req.TTL.Get(); ok {
		if v == "" {
			p.ClearTTL = true
		} else {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return nil, errs.Invalid("ttl is not a positive duration")
			}
			p.TTL = &d
		}
	}
	if v, ok := req.Scope.Get(); ok {
		sc := share.Scope(v)
		p.Scope = &sc
	}
	s, err := h.deps.Shares.Update(ctx, a, t.ID, params.ID, p)
	if err != nil {
		return nil, err
	}
	return h.shareOf(s), nil
}

// RebuildShare — re-capture the snapshot.
func (h *Handler) RebuildShare(ctx context.Context, params oas.RebuildShareParams) (*oas.Share, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	s, err := h.deps.Shares.Rebuild(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.shareOf(s), nil
}

// RevokeShare — the link stops working.
func (h *Handler) RevokeShare(ctx context.Context, params oas.RevokeShareParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Shares.Revoke(ctx, a, t.ID, params.ID)
}

// --- public -----------------------------------------------------------------

func sharedRunOf(r share.SharedRun) oas.SharedRun {
	out := oas.SharedRun{
		Name: r.Name, Status: oas.RunStatus(r.Status), Summary: summaryOf(r.Summary), Result: resultOf(r.Result), Timeline: make([]oas.RunEvent, 0, len(r.Timeline)),
		Machines: make([]oas.SharedRunMachinesItem, 0, len(r.Machines)), WorkloadSegments: []oas.SharedRunWorkloadSegmentsItem{},
		Database: oas.NewOptSharedRunDatabase(oas.SharedRunDatabase{Kind: oas.NewOptDatabaseKind(oas.DatabaseKind(r.Database.Kind)), Version: oas.NewOptString(r.Database.Version)}),
		Workload: oas.NewOptSharedRunWorkload(oas.SharedRunWorkload{StroppyVersion: oas.NewOptString(r.Workload.StroppyVersion), Protocol: oas.NewOptProtocol(oas.Protocol(r.Workload.Protocol)), Segments: segmentsTo(r.Workload.Segments)}),
	}
	if r.StartedAt != nil {
		out.StartedAt = oas.NewOptDateTime(*r.StartedAt)
	}
	if r.FinishedAt != nil {
		out.FinishedAt = oas.NewOptDateTime(*r.FinishedAt)
	}
	if r.Duration != nil {
		out.Duration = oas.NewOptString(time.Duration(*r.Duration * float64(time.Second)).Round(time.Second).String())
	}
	if r.Database.Image != "" {
		db := out.Database.Value
		db.Image = oas.NewOptString(r.Database.Image)
		out.Database = oas.NewOptSharedRunDatabase(db)
	}
	if len(r.Database.Params) > 0 {
		db := out.Database.Value
		db.Params = oas.NewOptSchemaValue(schemaValueOf(r.Database.Params))
		out.Database = oas.NewOptSharedRunDatabase(db)
	}
	for _, e := range r.Timeline {
		out.Timeline = append(out.Timeline, runEventOf(e))
	}
	for _, m := range r.Machines {
		out.Machines = append(out.Machines, oas.SharedRunMachinesItem{Name: oas.NewOptString(m.Name), Role: oas.NewOptString(m.Role), Size: oas.NewOptSize(oas.Size(m.Size)), CPU: oas.NewOptInt(m.CPU), MemoryGB: oas.NewOptInt(m.MemoryGB), DiskGB: oas.NewOptInt(m.DiskGB), DiskType: oas.NewOptString(m.DiskType), InstanceType: oas.NewOptString(m.InstanceType), Location: oas.NewOptString(m.Location)})
	}
	for _, s := range r.Segments {
		item := oas.SharedRunWorkloadSegmentsItem{Name: oas.NewOptString(s.Name)}
		if s.StartedAt != nil {
			item.StartedAt = oas.NewOptDateTime(*s.StartedAt)
		}
		if s.FinishedAt != nil {
			item.FinishedAt = oas.NewOptDateTime(*s.FinishedAt)
		}
		out.WorkloadSegments = append(out.WorkloadSegments, item)
	}
	if len(r.Configs) > 0 {
		c := oas.SharedRunConfigs{}
		for role, byID := range r.Configs {
			c[role] = oas.SharedRunConfigsItem(byID)
		}
		out.Configs = oas.NewOptSharedRunConfigs(c)
	}
	return out
}

func (h *Handler) snapshotOf(s share.Share) oas.ShareSnapshot {
	snap := s.Snapshot
	out := oas.ShareSnapshot{Kind: oas.ShareTargetKind(snap.Kind), Scope: oas.ShareScope(snap.Scope), CapturedAt: snap.CapturedAt}
	if snap.Title != "" {
		out.Title = oas.NewOptString(snap.Title)
	}
	if snap.TenantName != "" {
		out.TenantName = oas.NewOptString(snap.TenantName)
	}
	if snap.Run != nil {
		out.Run = oas.NewOptSharedRun(sharedRunOf(*snap.Run))
	}
	if snap.SuiteRun != nil {
		sr := oas.ShareSnapshotSuiteRun{Name: oas.NewOptString(snap.SuiteRun.Name), Status: oas.NewOptRunStatus(oas.RunStatus(snap.SuiteRun.Status)), Cells: make([]oas.SharedRun, 0, len(snap.SuiteRun.Cells))}
		for _, c := range snap.SuiteRun.Cells {
			sr.Cells = append(sr.Cells, sharedRunOf(c))
		}
		out.SuiteRun = oas.NewOptShareSnapshotSuiteRun(sr)
	}
	if snap.Comparison != nil {
		runs := map[uuid.UUID]run.Run{}
		for _, c := range snap.Comparison.Comparison.Columns {
			runs[c.Run.ID] = c.Run
		}
		out.Comparison = oas.NewOptComparison(comparisonOf(snap.Comparison.Comparison))
	}
	return out
}

// GetPublicShare — the lab report behind a token.
func (h *Handler) GetPublicShare(ctx context.Context, params oas.GetPublicShareParams) (*oas.ShareSnapshotHeaders, error) {
	s, err := h.deps.Shares.Public(ctx, params.Token)
	if err != nil {
		return nil, err
	}
	return &oas.ShareSnapshotHeaders{XRobotsTag: oas.NewOptString("noindex"), Response: h.snapshotOf(s)}, nil
}

// GetPublicShareMetrics exposes catalog series within the published run window.
func (h *Handler) GetPublicShareMetrics(ctx context.Context, params oas.GetPublicShareMetricsParams) (*oas.RunMetrics, error) {
	series, perKey, window, err := h.deps.Observe.SharedMetrics(ctx, h.deps.Shares, params.Token, params.Start.Or(time.Time{}), params.End.Or(time.Time{}))
	if err != nil {
		return nil, err
	}
	out := &oas.RunMetrics{Window: oas.RunMetricsWindow{Start: window.Start, End: window.End}, Series: seriesOf(series), Errors: []oas.RunMetricsErrorsItem{}}
	for _, err := range perKey {
		out.Errors = append(out.Errors, oas.RunMetricsErrorsItem{Error: oas.NewOptString(err.Error())})
	}
	return out, nil
}

// ExportPublicShare — the report as a document.
func (h *Handler) ExportPublicShare(ctx context.Context, params oas.ExportPublicShareParams) (oas.ExportPublicShareRes, error) {
	s, err := h.deps.Shares.Public(ctx, params.Token)
	if err != nil {
		return nil, err
	}
	switch params.Format {
	case oas.ExportFormatJSON:
		raw, err := json.Marshal(h.snapshotOf(s))
		if err != nil {
			return nil, err
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		out := oas.ExportPublicShareOKApplicationJSON{}
		for k, v := range m {
			out[k] = jx.Raw(v)
		}
		return &out, nil
	case oas.ExportFormatMd:
		b := &bytesBuilder{}
		b.printf("# %s\n\n", s.Title)
		if s.Snapshot.Run != nil {
			r := s.Snapshot.Run
			b.printf("- status: %s\n- database: %s %s\n- workload: %s (%s)\n- topology: %s\n", r.Status, r.Database.Kind, r.Database.Version, r.Summary.WorkloadName, r.Workload.Protocol, r.Topology)
			if len(r.Summary.Headline) > 0 {
				b.printf("\n| metric | value |\n|---|---|\n")
				for k, v := range r.Summary.Headline {
					b.printf("| %s | %g |\n", k, v)
				}
			}
		}
		return &oas.ExportPublicShareOKTextMarkdown{Data: b}, nil
	case oas.ExportFormatCsv:
		b := &bytesBuilder{}
		b.printf("metric,value\n")
		if s.Snapshot.Run != nil {
			for k, v := range s.Snapshot.Run.Summary.Headline {
				b.printf("%s,%g\n", k, v)
			}
		}
		return &oas.ExportPublicShareOKTextCsv{Data: b}, nil
	case oas.ExportFormatPdf:
		return nil, errs.Invalid("pdf export is not available yet")
	default:
		return nil, errs.Invalid("format " + string(params.Format) + " is not available yet")
	}
}
