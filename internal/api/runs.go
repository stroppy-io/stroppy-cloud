package api

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/go-faster/jx"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/share"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// --- wire mapping -----------------------------------------------------------

func bakedValueOf(schema, version string, raw json.RawMessage) oas.BakedValue {
	raw = browserSchemaJSON(raw)
	out := oas.BakedValue{Schema: oas.BakedValueSchema{"id": jx.Raw(strconv.Quote(schema)), "version": jx.Raw(strconv.Quote(version))}, Values: oas.BakedValueValues{}}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		for k, v := range m {
			out.Values[k] = jx.Raw(v)
		}
	}
	return out
}

func optTime(t *time.Time) oas.OptNilDateTime {
	if t == nil {
		return oas.OptNilDateTime{Null: true, Set: true}
	}
	return oas.NewOptNilDateTime(*t)
}

func optUUID(id *uuid.UUID) oas.OptUUID {
	if id == nil {
		return oas.OptUUID{}
	}
	return oas.NewOptUUID(*id)
}

func snapshotOf(s run.Snapshot) oas.RunSnapshot {
	out := oas.RunSnapshot{
		Database: specToWire(s.Database), Workload: workloadSpecToWire(s.Workload), Sizes: sizesTo(s.Sizes),
		ProviderProfile: oas.Ref{ID: s.ProviderProfile.ID, Name: oas.NewOptString(s.ProviderProfile.Name)},
		Machines:        make([]oas.RunSnapshotMachinesItem, 0, len(s.Machines)),
	}
	if len(s.Execution) > 0 {
		out.Execution = oas.NewOptSchemaValue(schemaValueOf(s.Execution))
	}
	if s.DatabaseName != "" {
		out.DatabaseName = oas.NewOptString(s.DatabaseName)
	}
	if s.WorkloadName != "" {
		out.WorkloadName = oas.NewOptString(s.WorkloadName)
	}
	if s.Keep != "" {
		out.Keep = oas.NewOptString(s.Keep)
	}
	if len(s.EffectiveConfigs) > 0 {
		ec := oas.RunSnapshotEffectiveConfigs{}
		for role, byID := range s.EffectiveConfigs {
			ec[role] = oas.RunSnapshotEffectiveConfigsItem{}
			for id, raw := range byID {
				ec[role][id] = bakedValueOf(schemaIDOf(id), schemaVersionOf(id), raw)
			}
		}
		out.EffectiveConfigs = oas.NewOptRunSnapshotEffectiveConfigs(ec)
	}
	for _, m := range s.Machines {
		item := oas.RunSnapshotMachinesItem{Name: m.Name, Role: m.Role, Size: oas.Size(m.Size), CPU: oas.NewOptInt(m.CPU), MemoryGB: oas.NewOptInt(m.MemoryGB)}
		if m.DiskGB > 0 {
			item.DiskGB = oas.NewOptInt(m.DiskGB)
		}
		if m.DiskType != "" {
			item.DiskType = oas.NewOptString(m.DiskType)
		}
		if m.InstanceType != "" {
			item.InstanceType = oas.NewOptString(m.InstanceType)
		}
		if m.Location != "" {
			item.Location = oas.NewOptString(m.Location)
		}
		out.Machines = append(out.Machines, item)
	}
	return out
}

// schemaIDOf / schemaVersionOf split "cfg.postgresql.conf@17".
func schemaIDOf(id string) string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '@' {
			return id[:i]
		}
	}
	return id
}

func schemaVersionOf(id string) string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '@' {
			return id[i+1:]
		}
	}
	return "1"
}

func summaryOf(s run.Summary) oas.RunSummary {
	out := oas.RunSummary{ProgressPct: oas.NewOptFloat64(s.ProgressPct)}
	if s.DBKind != "" {
		out.DbKind = oas.NewOptDatabaseKind(oas.DatabaseKind(s.DBKind))
	}
	if s.DBVersion != "" {
		out.DbVersion = oas.NewOptString(s.DBVersion)
	}
	if s.WorkloadName != "" {
		out.WorkloadName = oas.NewOptString(s.WorkloadName)
	}
	if s.Protocol != "" {
		out.Protocol = oas.NewOptProtocol(oas.Protocol(s.Protocol))
	}
	if s.StroppyVersion != "" {
		out.StroppyVersion = oas.NewOptString(s.StroppyVersion)
	}
	if s.TopologyLabel != "" {
		out.TopologyLabel = oas.NewOptString(s.TopologyLabel)
	}
	if s.NodeCount > 0 {
		out.NodeCount = oas.NewOptInt(s.NodeCount)
	}
	if s.ProviderKind != "" {
		out.ProviderKind = oas.NewOptProviderKind(oas.ProviderKind(s.ProviderKind))
	}
	if s.ProviderProfile != nil {
		out.ProviderProfile = oas.NewOptRef(oas.Ref{ID: s.ProviderProfile.ID, Name: oas.NewOptString(s.ProviderProfile.Name)})
	}
	if len(s.Sizes) > 0 {
		sizes := oas.RoleSizes{}
		for role, size := range s.Sizes {
			sizes[role] = oas.RoleSizesItem{Size: oas.Size(size)}
		}
		out.Sizes = oas.NewOptRoleSizes(sizes)
	}
	if s.Segment != "" {
		out.Segment = oas.NewOptString(s.Segment)
	}
	if len(s.Headline) > 0 {
		out.Headline = oas.NewOptRunSummaryHeadline(oas.RunSummaryHeadline(s.Headline))
	}
	return out
}

// resultOf uses the generated result decoder so API fields retain presence,
// zero values and opaque native reports. Only the historical status spelling
// differs between the pipeline and public API.
func resultOf(raw json.RawMessage) oas.OptRunResult {
	if len(raw) == 0 {
		return oas.OptRunResult{}
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) != nil || envelope == nil {
		return oas.OptRunResult{}
	}
	if segments, ok := envelope["segments"]; ok {
		var items []map[string]json.RawMessage
		if json.Unmarshal(segments, &items) != nil {
			return oas.OptRunResult{}
		}
		for _, item := range items {
			var status string
			if json.Unmarshal(item["status"], &status) == nil && status == "canceled" {
				item["status"] = json.RawMessage(`"cancelled"`)
			}
		}
		envelope["segments"], _ = json.Marshal(items) //nolint:errcheck // valid raw JSON
	}
	raw, _ = json.Marshal(envelope) //nolint:errcheck // valid raw JSON
	var out oas.RunResult
	if err := out.Decode(jx.DecodeBytes(raw)); err != nil {
		return oas.OptRunResult{}
	}
	if out.Segments == nil {
		out.Segments = []oas.RunResultSegmentsItem{}
	}
	if out.Artifacts == nil {
		out.Artifacts = []string{}
	}
	return oas.NewOptRunResult(out)
}

func (h *Handler) runOf(r run.Run, favorite *bool) *oas.Run {
	out := &oas.Run{
		ID: r.ID, Name: r.Name, Status: oas.RunStatus(r.Status), Phase: oas.RunPhase(r.Phase), StandKept: r.StandKept, KeepUntil: optTime(r.KeepUntil),
		Trigger: oas.Trigger(r.Trigger), Snapshot: snapshotOf(r.Snapshot),
		Rating: oas.RatingFlags{Tenant: oas.NewOptBool(r.RatingTenant), Global: oas.NewOptBool(r.RatingGlobal)},
		Labels: oas.NewOptRunLabels(oas.RunLabels(tagsOf(r.Labels))), CreatedAt: r.CreatedAt, StartedAt: optTime(r.StartedAt), FinishedAt: optTime(r.FinishedAt),
		Summary: oas.NewOptRunSummary(summaryOf(r.Summary)), Result: resultOf(r.Result), Shares: h.sharesOf(share.KindRun, r.ID),
		Graphene: oas.NewOptRunGraphene(oas.RunGraphene{RunRef: oas.NewOptString(r.GrapheneRef()), Namespace: oas.NewOptString(r.GrapheneNamespace)}),
	}
	if r.StatusReason != "" {
		out.StatusReason = oas.NewOptString(r.StatusReason)
	}
	if r.SuiteRunID != nil || r.ScheduleID != nil || r.ParentRunID != nil {
		ref := oas.RunTriggerRef{ScheduleID: optUUID(r.ScheduleID), SuiteRunID: optUUID(r.SuiteRunID), ParentRunID: optUUID(r.ParentRunID)}
		if r.CellID != "" {
			ref.CellID = oas.NewOptString(r.CellID)
		}
		out.TriggerRef = oas.NewOptRunTriggerRef(ref)
	}
	if r.TestID != nil {
		out.TestRef = oas.Ref{ID: *r.TestID, Name: oas.NewOptString(r.TestName)}
	}
	if len(r.RunSpec) > 0 {
		out.RunSpec = oas.NewOptBakedValue(bakedValueOf("spec.run", "1", r.RunSpec))
	}
	if r.Notes != "" {
		out.Notes = oas.NewOptString(r.Notes)
	}
	if r.AuthorID != nil {
		out.Author.ID = r.AuthorID.String()
	}
	if favorite != nil {
		out.IsFavorite = oas.NewOptBool(*favorite)
	}
	if r.DurationSeconds != nil {
		out.Duration = oas.NewOptNilString((time.Duration(*r.DurationSeconds * float64(time.Second))).Round(time.Second).String())
	} else {
		out.Duration = oas.OptNilString{Null: true, Set: true}
	}
	if r.PipelineRevision != "" {
		g := out.Graphene.Value
		g.PipelineRevision = oas.NewOptString(r.PipelineRevision)
		out.Graphene = oas.NewOptRunGraphene(g)
	}
	return out
}

func overridesOf(req oas.OptLaunchOverrides) (run.Overrides, error) {
	o := run.Overrides{}
	v, ok := req.Get()
	if !ok {
		return o, nil
	}
	o.Name = v.Name.Or("")
	if id, ok := v.ProviderProfileID.Get(); ok {
		o.ProviderProfileID = &id
	}
	o.Sizes = sizesFrom(v.Sizes)
	keep, err := keepFrom(v.Keep)
	if err != nil {
		return o, err
	}
	o.Keep = keep
	if r, ok := v.Rating.Get(); ok {
		if t, ok := r.Tenant.Get(); ok {
			o.RatingTenant = &t
		}
		if g, ok := r.Global.Get(); ok {
			o.RatingGlobal = &g
		}
	}
	if l, ok := v.Labels.Get(); ok {
		o.Labels = l
	}
	o.Notes = v.Notes.Or("")
	return o, nil
}

// favoriteOf resolves the actor's favorite flag for one run.
func (h *Handler) favoritesOf(ctx context.Context, a authActor, tenantID uuid.UUID) map[uuid.UUID]bool {
	favs, err := h.deps.Runs.Favorites(ctx, a, tenantID, "run")
	if err != nil {
		return map[uuid.UUID]bool{}
	}
	return favs
}

// --- launch -----------------------------------------------------------------

// LaunchTest — start a run of a test.
func (h *Handler) LaunchTest(ctx context.Context, req oas.OptLaunchOverrides, params oas.LaunchTestParams) (*oas.Run, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	o, err := overridesOf(req)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Runs.Launch(ctx, a, t.ID, params.ID, o, params.IdempotencyKey.Or(""))
	if err != nil {
		return nil, err
	}
	return h.runOf(r, nil), nil
}

// RerunRun — new run from a snapshot.
func (h *Handler) RerunRun(ctx context.Context, req oas.OptLaunchOverrides, params oas.RerunRunParams) (*oas.Run, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	o, err := overridesOf(req)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Runs.Rerun(ctx, a, t.ID, params.ID, o, params.IdempotencyKey.Or(""))
	if err != nil {
		return nil, err
	}
	return h.runOf(r, nil), nil
}

// ResumeRun — rerun from the failed segment; degrades to a rerun when the
// stand is gone (resumed=false).
func (h *Handler) ResumeRun(ctx context.Context, req oas.OptResumeRunReq, params oas.ResumeRunParams) (*oas.ResumeRunCreated, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	from := ""
	if v, ok := req.Get(); ok {
		from = v.FromSegment.Or("")
	}
	r, resumed, err := h.deps.Runs.Resume(ctx, a, t.ID, params.ID, from, params.IdempotencyKey.Or(""))
	if err != nil {
		return nil, err
	}
	base := h.runOf(r, nil)
	raw, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	out := &oas.ResumeRunCreated{}
	if err := out.UnmarshalJSON(raw); err != nil {
		return nil, err
	}
	out.Resumed = oas.NewOptBool(resumed)
	return out, nil
}

// --- reads ------------------------------------------------------------------

func runListQueryOf(p oas.ListRunsParams) (run.ListQuery, int, error) {
	offset, err := cursorOffset(p.Cursor)
	if err != nil {
		return run.ListQuery{}, 0, err
	}
	lim := p.Limit.Or(50)
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	q := run.ListQuery{
		Search: p.Search.Or(""), AuthorID: p.Author.Or(""), Labels: tagsFilterOf(p.Labels), Standalone: p.Standalone.Or(false), StandKeptOnly: p.StandKept.Or(false),
		Sort: string(p.Sort.Or(oas.ListRunsSortCreatedAt)), Desc: p.Order.Or(oas.OrderDesc) == oas.OrderDesc, Limit: lim + 1, Offset: offset,
	}
	for _, s := range p.Status {
		q.Statuses = append(q.Statuses, string(s))
	}
	for _, k := range p.Kind {
		q.Kinds = append(q.Kinds, string(k))
	}
	for _, id := range p.ProviderProfile {
		q.Profiles = append(q.Profiles, id.String())
	}
	for _, tr := range p.Trigger {
		q.Triggers = append(q.Triggers, string(tr))
	}
	if v, ok := p.TestID.Get(); ok {
		q.TestID = v.String()
	}
	if v, ok := p.SuiteRunID.Get(); ok {
		q.SuiteRunID = v.String()
	}
	if v, ok := p.StartedAfter.Get(); ok {
		q.StartedAfter = v
	}
	if v, ok := p.StartedBefore.Get(); ok {
		q.StartedBefore = v
	}
	if v, ok := p.FinishedAfter.Get(); ok {
		q.FinishedAfter = v
	}
	if v, ok := p.FinishedBefore.Get(); ok {
		q.FinishedBefore = v
	}
	if v, ok := p.DurationMin.Get(); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return q, 0, errs.Invalid("duration_min is not a duration")
		}
		q.DurationMin = d
	}
	if v, ok := p.DurationMax.Get(); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return q, 0, errs.Invalid("duration_max is not a duration")
		}
		q.DurationMax = d
	}
	if p.Favorites.Or(false) {
		q.FavoritesOf = "me"
	}
	return q, lim, nil
}

// ListRuns — filtered, sorted, paged.
func (h *Handler) ListRuns(ctx context.Context, params oas.ListRunsParams) (*oas.ListRunsOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	q, limit, err := runListQueryOf(params)
	if err != nil {
		return nil, err
	}
	list, err := h.deps.Runs.List(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, q.Offset, limit)
	favs := h.favoritesOf(ctx, a, t.ID)
	out := &oas.ListRunsOK{Data: make([]oas.Run, 0, len(list)), Meta: meta}
	for _, r := range list {
		f := favs[r.ID]
		out.Data = append(out.Data, *h.runOf(r, &f))
	}
	return out, nil
}

// GetRunFacets — filter value counts.
func (h *Handler) GetRunFacets(ctx context.Context, params oas.GetRunFacetsParams) (*oas.GetRunFacetsOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	facets, err := h.deps.Runs.Facets(ctx, a, t.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.GetRunFacetsOK{Data: make([]oas.Facet, 0, len(facets))}
	for _, f := range facets {
		item := oas.Facet{Field: f.Field, Values: make([]oas.FacetValuesItem, 0, len(f.Values))}
		for _, v := range f.Values {
			item.Values = append(item.Values, oas.FacetValuesItem{Value: v.Value, Count: v.Count})
		}
		out.Data = append(out.Data, item)
	}
	return out, nil
}

// ListTestRuns — history of one test.
func (h *Handler) ListTestRuns(ctx context.Context, params oas.ListTestRunsParams) (*oas.TestRunHistory, error) {
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
	list, err := h.deps.Runs.OfTest(ctx, a, t.ID, params.ID, limit+1, offset)
	if err != nil {
		return nil, err
	}
	list, meta := page(list, offset, limit)
	out := &oas.TestRunHistory{Data: make([]oas.Run, 0, len(list)), Meta: meta}
	trend := oas.TestRunHistoryTrend{Metric: oas.NewOptString("tps"), Points: []oas.TestRunHistoryTrendPointsItem{}}
	for _, r := range list {
		out.Data = append(out.Data, *h.runOf(r, nil))
		if r.TPS != nil && r.FinishedAt != nil {
			trend.Points = append(trend.Points, oas.TestRunHistoryTrendPointsItem{RunID: r.ID, At: *r.FinishedAt, Value: *r.TPS})
		}
	}
	out.Trend = oas.NewOptTestRunHistoryTrend(trend)
	return out, nil
}

// GetRun — one run.
func (h *Handler) GetRun(ctx context.Context, params oas.GetRunParams) (*oas.Run, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Runs.Get(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	f := h.favoritesOf(ctx, a, t.ID)[r.ID]
	return h.runOf(r, &f), nil
}

// PatchRun — name, notes, labels, rating.
func (h *Handler) PatchRun(ctx context.Context, req *oas.RunPatch, params oas.PatchRunParams) (*oas.Run, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p := run.MetaPatch{}
	if v, ok := req.Name.Get(); ok {
		p.Name = &v
	}
	if v, ok := req.Notes.Get(); ok {
		p.Notes = &v
	}
	if v, ok := req.Labels.Get(); ok {
		p.Labels = v
	}
	if r, ok := req.Rating.Get(); ok {
		if v, ok := r.Tenant.Get(); ok {
			p.RatingTenant = &v
		}
		if v, ok := r.Global.Get(); ok {
			p.RatingGlobal = &v
		}
	}
	r, err := h.deps.Runs.Patch(ctx, a, t.ID, params.ID, p)
	if err != nil {
		return nil, err
	}
	return h.runOf(r, nil), nil
}

// CancelRun — ask Graphene to stop.
func (h *Handler) CancelRun(ctx context.Context, params oas.CancelRunParams) (*oas.Run, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Runs.Cancel(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.runOf(r, nil), nil
}

// DeleteRun — soft delete + Graphene cascade.
func (h *Handler) DeleteRun(ctx context.Context, params oas.DeleteRunParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Runs.Delete(ctx, a, t.ID, params.ID)
}

// ExtendRunKeep — keep the stand longer.
func (h *Handler) ExtendRunKeep(ctx context.Context, req *oas.ExtendRunKeepReq, params oas.ExtendRunKeepParams) (*oas.Run, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	d, err := time.ParseDuration(req.Duration)
	if err != nil || d <= 0 {
		return nil, errs.Invalid("duration is not a positive duration")
	}
	r, err := h.deps.Runs.KeepExtend(ctx, a, t.ID, params.ID, d)
	if err != nil {
		return nil, err
	}
	return h.runOf(r, nil), nil
}

// ReleaseRunKeep — tear the stand down now.
func (h *Handler) ReleaseRunKeep(ctx context.Context, params oas.ReleaseRunKeepParams) (*oas.Run, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Runs.KeepRelease(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return h.runOf(r, nil), nil
}

// SaveRunAsTest — snapshot → test.
func (h *Handler) SaveRunAsTest(ctx context.Context, req *oas.SaveRunAsTestReq, params oas.SaveRunAsTestParams) (*oas.Test, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	test, fit, res, err := h.deps.Runs.SaveAsTest(ctx, a, t.ID, params.ID, req.Name, req.SaveDatabaseAs.Or(""), req.SaveWorkloadAs.Or(""))
	if err != nil {
		return nil, err
	}
	return h.testOf(test, fit, res), nil
}

// ListRunEvents — the timeline after an event id.
func (h *Handler) ListRunEvents(ctx context.Context, params oas.ListRunEventsParams) (*oas.ListRunEventsOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	after := int64(0)
	if v, ok := params.After.Get(); ok && v != "" {
		after, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, errs.Invalid("after is not an event id")
		}
	}
	limit := params.Limit.Or(200)
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	events, err := h.deps.Runs.Events(ctx, a, t.ID, params.ID, after, limit+1)
	if err != nil {
		return nil, err
	}
	meta := oas.PageMeta{HasMore: oas.NewOptBool(false)}
	if len(events) > limit {
		events = events[:limit]
		meta.HasMore = oas.NewOptBool(true)
		meta.NextCursor = oas.NewOptNilString(strconv.FormatInt(events[len(events)-1].ID, 10))
	}
	out := &oas.ListRunEventsOK{Data: make([]oas.RunEvent, 0, len(events)), Meta: meta}
	for _, e := range events {
		out.Data = append(out.Data, runEventOf(e))
	}
	return out, nil
}

func runEventOf(e run.Event) oas.RunEvent {
	item := oas.RunEvent{ID: strconv.FormatInt(e.ID, 10), At: e.At, Kind: e.Kind, Title: e.Title}
	if e.Subject != "" {
		item.Subject = oas.NewOptString(e.Subject)
	}
	if e.Status != "" {
		item.Status = oas.NewOptString(e.Status)
	}
	if e.Error != "" {
		item.Error = oas.NewOptString(e.Error)
	}
	if e.Attempt > 0 {
		item.Attempt = oas.NewOptInt(e.Attempt)
	}
	if len(e.Payload) > 0 {
		p := oas.RunEventPayload{}
		for k, v := range e.Payload {
			if raw, err := json.Marshal(v); err == nil {
				p[k] = jx.Raw(raw)
			}
		}
		item.Payload = oas.NewOptRunEventPayload(p)
	}
	return item
}

// GetRunOverview — the projected state (persisted; live comes over WS).
func (h *Handler) GetRunOverview(ctx context.Context, params oas.GetRunOverviewParams) (*oas.RunOverview, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Runs.Get(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return overviewOf(r), nil
}

func overviewOf(r run.Run) *oas.RunOverview {
	st := r.State
	source := oas.RunOverviewSourcePersisted
	if len(st.Phases) == 0 {
		source = oas.RunOverviewSourceSynthetic
	}
	out := &oas.RunOverview{
		RunID: r.ID, Status: oas.RunStatus(r.Status), Phase: oas.RunPhase(r.Phase), ProgressPct: oas.NewOptFloat64(st.Progress(r.Phase)),
		Source: source, ObservedAt: st.ObservedAt, DegradedReasons: st.Degraded,
		Phases: []oas.RunOverviewPhasesItem{}, Components: []oas.RunOverviewComponentsItem{}, Machines: []oas.RunOverviewMachinesItem{}, Flows: []oas.RunOverviewFlowsItem{}, WorkloadSegments: []oas.RunOverviewWorkloadSegmentsItem{},
	}
	if out.DegradedReasons == nil {
		out.DegradedReasons = []string{}
	}
	if out.ObservedAt.IsZero() {
		out.ObservedAt = r.UpdatedAt
	}
	for _, p := range st.SortedPhases() {
		item := oas.RunOverviewPhasesItem{ID: oas.RunPhase(p.ID), Title: string(p.ID), Status: oas.RunOverviewPhasesItemStatus(p.Status), StartedAt: optTime(p.StartedAt), FinishedAt: optTime(p.FinishedAt), Steps: []oas.RunOverviewPhasesItemStepsItem{}}
		for _, s := range p.Steps {
			step := oas.RunOverviewPhasesItemStepsItem{ID: s.ID, Title: s.Title, Status: s.Status, StartedAt: optTime(s.StartedAt), FinishedAt: optTime(s.FinishedAt)}
			if s.Role != "" {
				step.Role = oas.NewOptString(s.Role)
			}
			if s.Machine != "" {
				step.Machine = oas.NewOptString(s.Machine)
			}
			if s.Attempt > 0 {
				step.Attempt = oas.NewOptInt(s.Attempt)
			}
			if s.Error != "" {
				step.Error = oas.NewOptString(s.Error)
			}
			item.Steps = append(item.Steps, step)
		}
		out.Phases = append(out.Phases, item)
	}
	for _, m := range r.Snapshot.Machines {
		ms := st.Machines[m.Name]
		item := oas.RunOverviewMachinesItem{Name: m.Name, Role: m.Role, Status: oas.RunOverviewMachinesItemStatus(orDefault(ms.Status, "pending")), Presence: oas.RunOverviewMachinesItemPresence(orDefault(ms.Presence, "offline")), Size: oas.NewOptSize(oas.Size(m.Size)), LastHeartbeatAt: optTime(ms.LastHeartbeatAt)}
		if ms.AgentID != "" {
			item.AgentID = oas.NewOptString(ms.AgentID)
		}
		if ms.PrivateIP != "" {
			item.Address = oas.NewOptString(ms.PrivateIP)
		}
		if ms.PublicIP != "" {
			item.PublicIP = oas.NewOptString(ms.PublicIP)
		}
		if ms.ProviderResourceID != "" {
			item.ProviderResourceID = oas.NewOptString(ms.ProviderResourceID)
		}
		out.Machines = append(out.Machines, item)
	}
	var runSpec struct {
		Containers []struct {
			Name    string `json:"name"`
			Role    string `json:"role"`
			Machine string `json:"machine"`
			Image   string `json:"image"`
			Scrape  string `json:"scrape"`
			Ports   []struct {
				Host int `json:"host"`
			} `json:"ports"`
		} `json:"containers"`
		Flows []struct {
			FromRole string `json:"from_role"`
			ToRole   string `json:"to_role"`
			Protocol string `json:"protocol"`
			Port     int    `json:"port"`
			Label    string `json:"label"`
		} `json:"flows"`
	}
	if len(r.RunSpec) > 0 {
		_ = json.Unmarshal(r.RunSpec, &runSpec) //nolint:errcheck // baked by us
	}
	for _, c := range runSpec.Containers {
		cs := st.Containers[c.Name]
		item := oas.RunOverviewComponentsItem{ID: c.Name, Role: c.Role, Machine: c.Machine, Image: oas.NewOptString(c.Image), Status: oas.RunOverviewComponentsItemStatus(orDefault(cs.Status, "pending")), Endpoints: []oas.RunOverviewComponentsItemEndpointsItem{}}
		if c.Scrape != "" {
			item.Scrape = oas.NewOptString(c.Scrape)
		}
		for _, p := range c.Ports {
			item.Endpoints = append(item.Endpoints, oas.RunOverviewComponentsItemEndpointsItem{Port: oas.NewOptInt(p.Host)})
		}
		out.Components = append(out.Components, item)
	}
	for _, f := range runSpec.Flows {
		proto := f.Label
		if proto == "" {
			proto = f.Protocol
		}
		out.Flows = append(out.Flows, oas.RunOverviewFlowsItem{From: oas.NewOptString(f.FromRole), To: oas.NewOptString(f.ToRole), Protocol: oas.NewOptString(proto), Port: oas.NewOptInt(f.Port)})
	}
	for _, s := range st.Segments {
		out.WorkloadSegments = append(out.WorkloadSegments, oas.RunOverviewWorkloadSegmentsItem{Name: s.Name, Status: s.Status, StartedAt: optTime(s.StartedAt), FinishedAt: optTime(s.FinishedAt)})
	}
	if st.Pending != nil {
		pa := oas.RunOverviewPendingActivity{Activity: oas.NewOptString(st.Pending.Activity), Attempt: oas.NewOptInt(st.Pending.Attempt), Since: oas.NewOptDateTime(st.Pending.Since)}
		if st.Pending.LastFailure != "" {
			pa.LastFailure = oas.NewOptString(st.Pending.LastFailure)
		}
		out.PendingActivity = oas.NewOptRunOverviewPendingActivity(pa)
	}
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// GetRunTree — raw Graphene resources under the run.
func (h *Handler) GetRunTree(ctx context.Context, params oas.GetRunTreeParams) (*oas.ResourceTree, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	node, err := h.deps.Runs.Tree(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	out := treeOf(node)
	return &out, nil
}

func treeOf(n run.TreeNode) oas.ResourceTree {
	out := oas.ResourceTree{Ref: n.Ref, Kind: n.Kind, KeepUntil: optTime(n.KeepUntil), Children: []oas.ResourceTree{}}
	if n.Phase != "" {
		out.Phase = oas.NewOptString(n.Phase)
	}
	if len(n.Labels) > 0 {
		out.Labels = oas.NewOptResourceTreeLabels(oas.ResourceTreeLabels(n.Labels))
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, treeOf(c))
	}
	return out
}

// ListRunArtifacts — downloadable outputs.
func (h *Handler) ListRunArtifacts(ctx context.Context, params oas.ListRunArtifactsParams) (*oas.ListRunArtifactsOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	arts, err := h.deps.Runs.Artifacts(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.ListRunArtifactsOK{Data: make([]oas.Artifact, 0, len(arts))}
	for _, x := range arts {
		item := oas.Artifact{ID: x.ID(), Name: x.Name, Kind: artifactKindOf(x.Kind), SizeBytes: int(x.SizeBytes)}
		if x.ContentType != "" {
			item.ContentType = oas.NewOptString(x.ContentType)
		}
		if x.Digest != "" {
			item.Digest = oas.NewOptString(x.Digest)
		}
		if x.CreatedAt != nil {
			item.CreatedAt = oas.NewOptDateTime(*x.CreatedAt)
		}
		out.Data = append(out.Data, item)
	}
	return out, nil
}

func artifactKindOf(kind string) oas.ArtifactKind {
	switch k := oas.ArtifactKind(kind); k {
	case oas.ArtifactKindStroppyRaw, oas.ArtifactKindReport, oas.ArtifactKindConfig, oas.ArtifactKindLogBundle, oas.ArtifactKindOther:
		return k
	default:
		return oas.ArtifactKindOther
	}
}

// DownloadRunArtifact — stream one artifact.
func (h *Handler) DownloadRunArtifact(ctx context.Context, params oas.DownloadRunArtifactParams) (oas.DownloadRunArtifactOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return oas.DownloadRunArtifactOK{}, err
	}
	_, rc, err := h.deps.Runs.Download(ctx, a, t.ID, params.ID, params.ArtifactId)
	if err != nil {
		return oas.DownloadRunArtifactOK{}, err
	}
	return oas.DownloadRunArtifactOK{Data: rc}, nil
}

// ExportRun — the run as a document.
func (h *Handler) ExportRun(ctx context.Context, params oas.ExportRunParams) (oas.ExportRunRes, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	r, err := h.deps.Runs.Get(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	switch params.Format {
	case oas.ExportFormatJSON:
		raw, err := json.Marshal(h.runOf(r, nil))
		if err != nil {
			return nil, err
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		out := oas.ExportRunOKApplicationJSON{}
		for k, v := range m {
			out[k] = jx.Raw(v)
		}
		return &out, nil
	case oas.ExportFormatMd:
		return &oas.ExportRunOKTextMarkdown{Data: runMarkdown(r)}, nil
	case oas.ExportFormatCsv:
		return &oas.ExportRunOKTextCsv{Data: runCSV(r)}, nil
	case oas.ExportFormatPdf:
		return nil, errs.Invalid("pdf export is not available yet")
	default:
		return nil, errs.Invalid("format " + string(params.Format) + " is not available yet")
	}
}

func runMarkdown(r run.Run) io.Reader {
	b := &bytesBuilder{}
	b.printf("# %s\n\n", r.Name)
	b.printf("- status: %s (%s)\n- database: %s %s\n- workload: %s (%s)\n- topology: %s, %d machines\n- provider: %s\n",
		r.Status, r.Phase, r.Summary.DBKind, r.Summary.DBVersion, r.Summary.WorkloadName, r.Summary.Protocol, r.Summary.TopologyLabel, r.Summary.NodeCount, r.Summary.ProviderKind)
	if len(r.Summary.Headline) > 0 {
		b.printf("\n## Headline\n\n| metric | value |\n|---|---|\n")
		for k, v := range r.Summary.Headline {
			b.printf("| %s | %g |\n", k, v)
		}
	}
	if r.Notes != "" {
		b.printf("\n## Notes\n\n%s\n", r.Notes)
	}
	return b
}

func runCSV(r run.Run) io.Reader {
	b := &bytesBuilder{}
	b.printf("metric,value\n")
	b.printf("status,%s\nphase,%s\n", r.Status, r.Phase)
	for k, v := range r.Summary.Headline {
		b.printf("%s,%g\n", k, v)
	}
	return b
}

// --- favorites --------------------------------------------------------------

// AddFavorite — mark.
func (h *Handler) AddFavorite(ctx context.Context, params oas.AddFavoriteParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Runs.SetFavorite(ctx, a, t.ID, string(params.Kind), params.ID, true)
}

// RemoveFavorite — unmark.
func (h *Handler) RemoveFavorite(ctx context.Context, params oas.RemoveFavoriteParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Runs.SetFavorite(ctx, a, t.ID, string(params.Kind), params.ID, false)
}

var _ = library.Test{}
