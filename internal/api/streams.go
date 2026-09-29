package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// Streams is the WebSocket source over the same mappers the REST
// endpoints use: run.overview/{id}, run.events/{id}, suite_run/{id},
// tenant.runs/{slug}, run.logs/{id}, run.metrics/{id}. PTY is not connected.
type Streams struct{ h *Handler }

// NewStreams builds the source.
func NewStreams(h *Handler) *Streams { return &Streams{h: h} }

// IsEvents reports the cursor-based topics.
func (s *Streams) IsEvents(topic string) bool {
	return strings.HasPrefix(topic, "run.events/") || strings.HasPrefix(topic, "run.logs/")
}

func splitTopic(topic string) (kind, arg string, err error) {
	kind, arg, ok := strings.Cut(topic, "/")
	if !ok || arg == "" {
		return "", "", errs.Invalid("topic must be <kind>/<id>")
	}
	if i := strings.IndexByte(arg, '?'); i >= 0 {
		arg = arg[:i]
	}
	return kind, arg, nil
}

// runOf resolves a run the actor may read (its tenant membership).
func (s *Streams) runOf(ctx context.Context, actor auth.Actor, id string) (run.Run, error) {
	rid, err := uuid.Parse(id)
	if err != nil {
		return run.Run{}, errs.Invalid("run id is not a uuid")
	}
	r, err := s.h.deps.Runs.Peek(ctx, rid)
	if err != nil {
		return run.Run{}, err
	}
	if _, _, err := s.h.deps.Tenants.RoleIn(ctx, actor, r.TenantID); err != nil {
		return run.Run{}, errs.NotFound("run")
	}
	return r, nil
}

// Snapshot answers a state topic; UserRefs in the payload are filled the
// same way the REST middleware fills them.
func (s *Streams) Snapshot(ctx context.Context, actor auth.Actor, topic string) (payload any, version string, err error) {
	payload, version, err = s.snapshot(ctx, actor, topic)
	if err != nil {
		return nil, "", err
	}
	return s.h.fillUsers(auth.WithActor(ctx, actor), payload), version, nil
}

func (s *Streams) snapshot(ctx context.Context, actor auth.Actor, topic string) (payload any, version string, err error) {
	kind, arg, err := splitTopic(topic)
	if err != nil {
		return nil, "", err
	}
	switch kind {
	case "run.overview":
		r, err := s.runOf(ctx, actor, arg)
		if err != nil {
			return nil, "", err
		}
		return overviewOf(r), fmt.Sprintf("%d/%s/%s", r.LastEventID, r.Status, r.UpdatedAt.Format("20060102150405.000")), nil
	case "run":
		r, err := s.runOf(ctx, actor, arg)
		if err != nil {
			return nil, "", err
		}
		return s.h.runOf(r, nil), r.UpdatedAt.Format("20060102150405.000"), nil
	case "suite_run":
		id, err := uuid.Parse(arg)
		if err != nil {
			return nil, "", errs.Invalid("suite run id is not a uuid")
		}
		sr, err := s.h.deps.Suites.Peek(ctx, id)
		if err != nil {
			return nil, "", err
		}
		if _, _, err := s.h.deps.Tenants.RoleIn(ctx, actor, sr.TenantID); err != nil {
			return nil, "", errs.NotFound("suite run")
		}
		view := s.h.suiteRunOf(sr)
		raw, _ := json.Marshal(view) //nolint:errcheck // struct
		return view, strconv.Itoa(len(raw)) + "/" + string(sr.Status) + "/" + fmt.Sprint(view.Progress), nil
	case "tenant.runs":
		t, err := s.h.deps.Tenants.Get(ctx, actor, arg)
		if err != nil {
			return nil, "", err
		}
		list, err := s.h.deps.Runs.List(ctx, actor, t.ID, run.ListQuery{Statuses: []string{"pending", "running", "cancelling"}, Sort: "created_at", Desc: true, Limit: 100})
		if err != nil {
			return nil, "", err
		}
		out := make([]any, 0, len(list))
		version := strings.Builder{}
		for _, r := range list {
			out = append(out, s.h.runOf(r, nil))
			version.WriteString(r.ID.String()[:8] + ":" + string(r.Status) + ":" + string(r.Phase) + ";")
		}
		return map[string]any{"data": out}, version.String(), nil
	case "run.metrics":
		r, err := s.runOf(ctx, actor, arg)
		if err != nil {
			return nil, "", err
		}
		series, failures, w, err := s.h.deps.Observe.Metrics(ctx, actor, r.TenantID, r.ID, nil, "", time.Time{}, time.Time{}, 0)
		if err != nil {
			return nil, "", err
		}
		out := &oas.RunMetrics{Window: oas.RunMetricsWindow{Start: w.Start, End: w.End}, Series: seriesOf(series), Errors: []oas.RunMetricsErrorsItem{}}
		for _, f := range failures {
			out.Errors = append(out.Errors, oas.RunMetricsErrorsItem{Error: oas.NewOptString(f.Error())})
		}
		return out, w.End.Format(time.RFC3339), nil
	case "agent.pty":
		return nil, "", errs.New(errs.CodeUnavailable, "pty streams are not connected yet")
	default:
		return nil, "", errs.Invalid("unknown topic " + kind)
	}
}

// Events answers run.events/{id} after a cursor (event id) and
// run.logs/{id} after an opaque Graphene-backed cursor: the tail.
func (s *Streams) Events(ctx context.Context, actor auth.Actor, topic, cursor string) (events []any, next string, err error) {
	kind, arg, err := splitTopic(topic)
	if err != nil {
		return nil, "", err
	}
	r, err := s.runOf(ctx, actor, arg)
	if err != nil {
		return nil, "", err
	}
	if kind == "run.logs" {
		q := observe.LogQuery{RunID: r.ID, Cursor: cursor, Direction: "newer", Limit: 200}
		if cursor == "" {
			q.Start = time.Now().UTC().Add(-time.Minute)
		}
		page, _, err := s.h.deps.Observe.Logs(ctx, actor, r.TenantID, q)
		if err != nil {
			return nil, "", err
		}
		out := make([]any, 0, len(page.Lines))
		// Oldest first for a tail.
		for i := len(page.Lines) - 1; i >= 0; i-- {
			line := logLineOf(page.Lines[i])
			out = append(out, &line)
		}
		next = q.Cursor
		if page.Newer != "" {
			next = page.Newer
		}
		return out, next, nil
	}
	after := int64(0)
	if cursor != "" {
		if after, err = strconv.ParseInt(cursor, 10, 64); err != nil {
			return nil, "", errs.Invalid("cursor is not an event id")
		}
	}
	list, err := s.h.deps.Runs.Events(ctx, actor, r.TenantID, r.ID, after, 500)
	if err != nil {
		return nil, "", err
	}
	out := make([]any, 0, len(list))
	next = cursor
	for _, e := range list {
		// Pointers: ogen's MarshalJSON has pointer receivers and the
		// optional fields do not survive encoding/json's fallback.
		ev := runEventOf(e)
		out = append(out, &ev)
		next = strconv.FormatInt(e.ID, 10)
	}
	return out, next, nil
}
