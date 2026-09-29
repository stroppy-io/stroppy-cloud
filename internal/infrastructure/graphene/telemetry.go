package graphene

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Logs and Metrics use the same authenticated Graphene door as run control.
// Every request carries both the tenant namespace and the run ref. Graphene
// authorizes and scopes the expression; this client never issues global reads.
type Logs struct{ c *Client }

// Metrics reads metric snapshots through Graphene.
type Metrics struct{ c *Client }

func NewLogs(c *Client) *Logs       { return &Logs{c: c} }
func NewMetrics(c *Client) *Metrics { return &Metrics{c: c} }

// logCursor keeps Graphene's page token opaque. An exhausted forward page
// is replayed on the next poll, skipping the records already delivered. This
// preserves equal timestamps without inventing a Graphene cursor. Older
// pages retain their original time window; forward pages extend its end.
type logCursor struct {
	Token     string        `json:"p,omitempty"`
	Skip      int           `json:"n,omitempty"`
	Start     int64         `json:"s"`
	End       int64         `json:"e,omitempty"`
	Direction string        `json:"d"`
	Scope     observe.Scope `json:"scope"`
	Filter    string        `json:"f"`
}

func encodeLogCursor(c logCursor) string {
	b, _ := json.Marshal(c) //nolint:errcheck // scalar fields
	return base64.RawURLEncoding.EncodeToString(b)
}

func typedLogFilter(q observe.LogQuery) string {
	var parts []string
	for _, f := range []struct {
		name   string
		values []string
	}{
		{spec.AttrAgent, q.Agents}, {spec.AttrEntity, q.Entities}, {"stream", q.Streams},
	} {
		if len(f.values) == 0 {
			continue
		}
		values := make([]string, len(f.values))
		for i, v := range f.values {
			values[i] = strconv.Quote(v)
		}
		parts = append(parts, fmt.Sprintf("%q:in(%s)", f.name, strings.Join(values, ",")))
	}
	return strings.Join(parts, " AND ")
}

func (l *Logs) Query(ctx context.Context, q observe.LogQuery) (observe.LogPage, error) {
	direction := q.Direction
	if direction == "" {
		direction = "older"
	}
	if direction != "older" && direction != "newer" {
		return observe.LogPage{}, errs.Invalid("invalid log direction")
	}
	req := &managementv1.LogsRequest{
		Ref: "run/" + q.Scope.Run, Query: typedLogFilter(q), Text: q.Text, Severities: q.Levels,
		SinceUnixNano: q.Start.UnixNano(), UntilUnixNano: q.End.UnixNano(), Order: "desc",
	}
	if direction == "newer" {
		req.Order = "asc"
	}
	identity, _ := json.Marshal([]any{req.Query, req.Text, req.Severities}) //nolint:errcheck // strings only
	cursor := logCursor{Start: req.SinceUnixNano, End: req.UntilUnixNano, Direction: direction, Scope: q.Scope, Filter: string(identity)}
	if q.Cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		if err != nil || json.Unmarshal(b, &cursor) != nil || cursor.Direction != direction || cursor.Scope != q.Scope || cursor.Filter != string(identity) || cursor.Skip < 0 || cursor.Skip > 10000 {
			return observe.LogPage{}, errs.Invalid("invalid log cursor for this selection")
		}
		req.PageToken, req.SinceUnixNano = cursor.Token, cursor.Start
		if direction == "older" {
			req.UntilUnixNano = cursor.End
		}
	}
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	// A line's seq as the cursor: the page starts from that line itself
	// and carries the cursor the other way, which starts next to it.
	var back logCursor
	known := map[int64][]observe.LogLine{}
	if !q.Anchor.IsZero() {
		if q.Cursor != "" {
			return observe.LogPage{}, errs.Invalid("a log cursor is either a page cursor or a line seq")
		}
		var err error
		if cursor, back, err = l.anchor(ctx, q, req, cursor, known); err != nil {
			return observe.LogPage{}, err
		}
		req.SinceUnixNano = cursor.Start
		if direction == "older" {
			req.UntilUnixNano = cursor.End
		}
	}
	req.Limit = int32(min(cursor.Skip+limit, 10000)) //nolint:gosec // bounded above
	page, next, err := l.read(ctx, q.Scope, req)
	if err != nil {
		return observe.LogPage{}, err
	}
	if cursor.Skip == len(page.Lines) && next != "" {
		cursor.Token, cursor.Skip = next, 0
		req.PageToken, req.Limit = next, int32(limit)
		page, next, err = l.read(ctx, q.Scope, req)
		if err != nil {
			return observe.LogPage{}, err
		}
	}
	read := len(page.Lines)
	page.Lines = page.Lines[min(cursor.Skip, read):]
	// The page starts where a cursor or a seq left off, and ends where more
	// records follow: only those edges can split a microsecond.
	continued := q.Cursor != "" || !q.Anchor.IsZero()
	if err := l.assignSeqs(ctx, q, req, page.Lines, direction == "newer", continued, next != "", known); err != nil {
		return observe.LogPage{}, err
	}
	if next != "" {
		cursor.Token, cursor.Skip = next, 0
	} else {
		cursor.Skip = read
	}
	if direction == "newer" {
		page.Newer = encodeLogCursor(cursor)
		slices.Reverse(page.Lines)
		if !q.Anchor.IsZero() {
			page.Older = encodeLogCursor(back)
		}
	} else {
		if next != "" {
			page.Older = encodeLogCursor(cursor)
		}
		if !q.Anchor.IsZero() {
			page.Newer = encodeLogCursor(back)
		}
	}
	return page, nil
}

// anchor turns a line's seq into the page's own cursor and the cursor the
// other way. It reads the lines of the seq's microsecond under the
// selection, in the store's order (time, stream, body), and finds the line
// by its digest: i is its index among the lines of its exact time, n their
// count. An older page skips the n-1-i lines after it (desc), the newer
// cursor from it the i+1 lines up to it (asc); a newer page skips i and
// the older cursor from it n-i. Our own skip over a bounded window — no
// Graphene token is forged. A line the selection does not hold anchors the
// edge of its microsecond: the page stays near it, with no line twice.
func (l *Logs) anchor(ctx context.Context, q observe.LogQuery, req *managementv1.LogsRequest, base logCursor, known map[int64][]observe.LogLine) (page, back logCursor, err error) {
	at, next := q.Anchor.At.UnixNano(), q.Anchor.Next().UnixNano()
	group, err := l.micro(ctx, q.Scope, req, observe.Micro(q.Anchor.At))
	if err != nil {
		return logCursor{}, logCursor{}, err
	}
	known[observe.Micro(q.Anchor.At)] = group
	t, i, n := int64(0), 0, 0
	for k, bucket := range observe.SeqBuckets(group) {
		if bucket != q.Anchor.Bucket {
			continue
		}
		t = group[k].Time.UnixNano()
		for _, other := range group[:k] {
			if other.Time.UnixNano() == t {
				i++
			}
		}
		for _, other := range group {
			if other.Time.UnixNano() == t {
				n++
			}
		}
		break
	}
	older, newer := base, base
	older.Direction, newer.Direction = "older", "newer"
	older.Token, newer.Token = "", ""
	if n == 0 {
		// Not in the selection: both cursors meet at one edge of the
		// microsecond (its end for an older page, its start for a newer
		// one), so the whole microsecond lands on the page, once.
		older.End, older.Skip = next, 0
		newer.Start, newer.Skip = next-1, 0
		if q.Direction == "newer" {
			older.End, newer.Start = at, at-1
		}
	} else {
		older.End, newer.Start = t+1, t-1
		older.Skip, newer.Skip = n-1-i, i+1
		if q.Direction == "newer" {
			older.Skip, newer.Skip = n-i, i
		}
	}
	older.Start = base.Start
	if q.Direction == "newer" {
		return newer, older, nil
	}
	return older, newer, nil
}

// micro reads one microsecond of the selection whole, in the store's
// order. The run's window does not bound it: a seq is the line's place in
// the store, whichever window the page was read under.
func (l *Logs) micro(ctx context.Context, scope observe.Scope, req *managementv1.LogsRequest, us int64) ([]observe.LogLine, error) {
	// Graphene's bounds are exclusive on both ends.
	probe := &managementv1.LogsRequest{Ref: req.Ref, Query: req.Query, Text: req.Text, Severities: req.Severities, SinceUnixNano: us*1000 - 1, UntilUnixNano: (us + 1) * 1000, Order: "asc", Limit: 10000}
	page, _, err := l.read(ctx, scope, probe)
	return page.Lines, err
}

// assignSeqs sets the seqs of a page in the store's order (asc) or
// reversed. A microsecond at an edge the page may split (near: where it
// continues a cursor; far: where more records follow) is read whole, so a
// line's seq does not depend on the page. known holds microseconds already
// read whole.
func (l *Logs) assignSeqs(ctx context.Context, q observe.LogQuery, req *managementv1.LogsRequest, lines []observe.LogLine, asc, near, far bool, known map[int64][]observe.LogLine) error {
	if len(lines) == 0 {
		return nil
	}
	var edges []int64
	for _, e := range []struct {
		split bool
		line  observe.LogLine
	}{{near, lines[0]}, {far, lines[len(lines)-1]}} {
		us := observe.Micro(e.line.Time)
		if _, ok := known[us]; e.split && !ok && !slices.Contains(edges, us) {
			edges = append(edges, us)
		}
	}
	groups := make([][]observe.LogLine, len(edges))
	failures := make([]error, len(edges))
	var wg sync.WaitGroup
	for i, us := range edges {
		wg.Go(func() { groups[i], failures[i] = l.micro(ctx, q.Scope, req, us) })
	}
	wg.Wait()
	for i, us := range edges {
		if failures[i] != nil {
			return failures[i]
		}
		known[us] = groups[i]
	}
	observe.AssignSeqs(q.Origin, lines, asc, known)
	return nil
}

func (l *Logs) read(ctx context.Context, scope observe.Scope, req *managementv1.LogsRequest) (observe.LogPage, string, error) {
	if scope.Namespace == "" || scope.Run == "" {
		return observe.LogPage{}, "", errs.Invalid("telemetry requires a run scope")
	}
	ctx, cancel := context.WithTimeout(WithNamespace(ctx, scope.Namespace), 30*time.Second)
	defer cancel()
	stream, err := l.c.Observe.Logs(ctx, connect.NewRequest(req))
	if err != nil {
		return observe.LogPage{}, "", telemetryError(err)
	}
	defer stream.Close()
	page := observe.LogPage{Lines: []observe.LogLine{}}
	var next string
	for stream.Receive() {
		chunk := stream.Msg()
		if rec := chunk.GetRecord(); rec != nil {
			fields := rec.GetAttributes()
			page.Lines = append(page.Lines, observe.LogLine{Time: time.Unix(0, rec.GetTimeUnixNano()).UTC(), Message: rec.GetBody(), Level: strings.ToLower(rec.GetSeverity()), Stream: fields["stream"], Fields: fields})
		}
		if p := chunk.GetPage(); p != nil {
			page.Truncated, next = p.GetTruncated(), p.GetNextPageToken()
		}
		if chunk.GetDropped() > 0 {
			return observe.LogPage{}, "", errs.New(errs.CodeUnavailable, "Graphene dropped log records")
		}
	}
	if err := stream.Err(); err != nil {
		return observe.LogPage{}, "", telemetryError(err)
	}
	return page, next, nil
}

func (l *Logs) Raw(ctx context.Context, scope observe.Scope, query string, start, end time.Time, limit int) (observe.LogPage, error) {
	page, _, err := l.read(ctx, scope, &managementv1.LogsRequest{Ref: "run/" + scope.Run, Query: query, SinceUnixNano: start.UnixNano(), UntilUnixNano: end.UnixNano(), Limit: int32(limit), Order: "desc"}) //nolint:gosec // service bounds limit
	return page, err
}

func (l *Logs) Facets(ctx context.Context, scope observe.Scope, start, end time.Time) ([]run.Facet, error) {
	if scope.Namespace == "" || scope.Run == "" {
		return nil, errs.Invalid("telemetry requires a run scope")
	}
	ctx, cancel := context.WithTimeout(WithNamespace(ctx, scope.Namespace), 30*time.Second)
	defer cancel()
	res, err := l.c.Observe.LogFacets(ctx, connect.NewRequest(&managementv1.LogFacetsRequest{
		Selection: &managementv1.LogsRequest{Ref: "run/" + scope.Run, SinceUnixNano: start.UnixNano(), UntilUnixNano: end.UnixNano()},
		Fields:    []string{spec.AttrAgent, spec.AttrEntity, "stream", "severity_text"}, Limit: 50,
	}))
	if err != nil {
		return nil, telemetryError(err)
	}
	out := []run.Facet{}
	for _, f := range res.Msg.GetFacets() {
		field := f.GetField()
		if field == "severity_text" || field == "severity" {
			field = "level"
		}
		facet := run.Facet{Field: field, Values: []run.FacetValue{}}
		for _, v := range f.GetValues() {
			value := v.GetValue()
			if field == "level" {
				value = strings.ToLower(value)
			}
			index := slices.IndexFunc(facet.Values, func(x run.FacetValue) bool { return x.Value == value })
			if index >= 0 {
				facet.Values[index].Count += int(v.GetHits())
			} else {
				facet.Values = append(facet.Values, run.FacetValue{Value: value, Count: int(v.GetHits())})
			}
		}
		out = append(out, facet)
	}
	return out, nil
}

func (m *Metrics) Query(ctx context.Context, q observe.MetricQuery) ([]observe.Series, []error, error) {
	var out []observe.Series
	var failures []error
	for _, def := range q.Metrics {
		raw, err := m.Raw(ctx, q.Scope, observe.KeyExpr(def, q.Scope), q.Start, q.End, q.Step)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", def.Key, err))
			continue
		}
		series, err := seriesOf(raw, def.Key, def.Title, def.Unit)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", def.Key, err))
			continue
		}
		out = append(out, series...)
	}
	return out, failures, nil
}

func (m *Metrics) Raw(ctx context.Context, scope observe.Scope, query string, start, end time.Time, step time.Duration) (json.RawMessage, error) {
	if scope.Namespace == "" || scope.Run == "" {
		return nil, errs.Invalid("telemetry requires a run scope")
	}
	seconds := math.Ceil(step.Seconds())
	if seconds < 0 || seconds > math.MaxInt32 {
		return nil, errs.Invalid("invalid metric step")
	}
	ctx, cancel := context.WithTimeout(WithNamespace(ctx, scope.Namespace), 30*time.Second)
	defer cancel()
	stream, err := m.c.Observe.Metrics(ctx, connect.NewRequest(&managementv1.MetricsRequest{Ref: "run/" + scope.Run, Query: query, StartUnixNano: start.UnixNano(), EndUnixNano: end.UnixNano(), StepSeconds: int32(seconds)}))
	if err != nil {
		return nil, telemetryError(err)
	}
	defer stream.Close()
	var snapshot json.RawMessage
	for stream.Receive() {
		if s := stream.Msg().GetSnapshot(); s != nil {
			snapshot = slices.Clone(s)
		}
		if stream.Msg().GetDropped() > 0 {
			return nil, errs.New(errs.CodeUnavailable, "Graphene dropped metric records")
		}
	}
	if err := stream.Err(); err != nil {
		return nil, telemetryError(err)
	}
	if !json.Valid(snapshot) {
		return nil, errs.New(errs.CodeUnavailable, "Graphene returned no valid metric snapshot")
	}
	return snapshot, nil
}

func telemetryError(err error) error {
	switch connect.CodeOf(err) { //nolint:exhaustive // remaining codes are upstream failures
	case connect.CodeInvalidArgument:
		return errs.Wrap(errs.CodeInvalid, "Graphene telemetry query", err)
	case connect.CodeNotFound:
		return errs.NotFound("Graphene telemetry run")
	case connect.CodePermissionDenied, connect.CodeUnauthenticated:
		return errs.Wrap(errs.CodeForbidden, "Graphene telemetry access", err)
	default:
		return errs.Wrap(errs.CodeUnavailable, "Graphene telemetry", err)
	}
}
