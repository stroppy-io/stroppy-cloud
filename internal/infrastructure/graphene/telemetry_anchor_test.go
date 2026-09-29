package graphene

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// storeDoor answers like Graphene over VictoriaLogs: (since, until)
// exclusive, records ordered by (time, stream, body), offset page tokens.
func storeDoor(records []*managementv1.LogRecord) observeDoor {
	sorted := slices.Clone(records)
	slices.SortStableFunc(sorted, func(a, b *managementv1.LogRecord) int {
		return cmp.Or(cmp.Compare(a.GetTimeUnixNano(), b.GetTimeUnixNano()), cmp.Compare(a.GetAttributes()["stream"], b.GetAttributes()["stream"]), cmp.Compare(a.GetBody(), b.GetBody()))
	})
	return observeDoor{logs: func(_ context.Context, r *connect.Request[managementv1.LogsRequest], s *connect.ServerStream[managementv1.LogChunk]) error {
		var sel []*managementv1.LogRecord
		for _, rec := range sorted {
			if rec.GetTimeUnixNano() > r.Msg.GetSinceUnixNano() && (r.Msg.GetUntilUnixNano() == 0 || rec.GetTimeUnixNano() < r.Msg.GetUntilUnixNano()) {
				sel = append(sel, rec)
			}
		}
		if r.Msg.GetOrder() == "desc" {
			slices.Reverse(sel)
		}
		start := 0
		if tok := r.Msg.GetPageToken(); tok != "" {
			start, _ = strconv.Atoi(tok) //nolint:errcheck // own token
		}
		end := min(start+int(r.Msg.GetLimit()), len(sel))
		start = min(start, end)
		for _, rec := range sel[start:end] {
			if err := s.Send(&managementv1.LogChunk{Chunk: &managementv1.LogChunk_Record{Record: rec}}); err != nil {
				return err
			}
		}
		page := &managementv1.LogPage{Returned: int32(end - start), Truncated: end < len(sel)} //nolint:gosec // tiny fixture
		if page.Truncated {
			page.NextPageToken = strconv.Itoa(end)
		}
		return s.Send(&managementv1.LogChunk{Chunk: &managementv1.LogChunk_Page{Page: page}})
	}}
}

func TestLogsSeqCursorAnchorsTheLine(t *testing.T) {
	created := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	origin := observe.SeqOrigin(run.Run{CreatedAt: created})
	base := created.Add(time.Minute)
	var records []*managementv1.LogRecord
	add := func(at time.Time, stream, body string) {
		records = append(records, &managementv1.LogRecord{TimeUnixNano: at.UnixNano(), Body: body, Severity: "INFO", Attributes: map[string]string{"stream": stream}})
	}
	for i := range 20 {
		add(base.Add(time.Duration(i)*time.Second), "stdout", "line-"+strconv.Itoa(i))
	}
	// A burst: five lines of one nanosecond, two more in its microsecond.
	burst := base.Add(10*time.Second + 500*time.Millisecond)
	for i := range 5 {
		add(burst, []string{"stderr", "stdout"}[i%2], "burst-"+strconv.Itoa(i))
	}
	add(burst.Add(-300), "stdout", "burst-early")
	add(burst.Add(200), "stdout", "burst-late")
	// Two lines of one digest in the burst's microsecond, one written twice.
	seen := map[uint16]string{}
	for i := 0; ; i++ {
		body := "twin-" + strconv.Itoa(i)
		h := observe.LineHash(observe.LogLine{Message: body, Level: "info", Fields: map[string]string{"stream": "stdout"}})
		if other, ok := seen[h]; ok {
			add(burst.Add(100), "stdout", other)
			add(burst.Add(100), "stdout", body)
			add(burst.Add(100), "stdout", body)
			break
		}
		seen[h] = body
	}

	scope := observe.Scope{Namespace: "tenant-a", Run: "run-a"}
	l := NewLogs(observeClient(t, storeDoor(records)))
	window := func() observe.LogQuery {
		return observe.LogQuery{Scope: scope, Origin: origin, Start: created, End: base.Add(time.Hour), Limit: 4}
	}
	// Everything, oldest first — the order the store answers in.
	all := func() []observe.LogLine {
		q := window()
		q.Direction, q.Limit = "newer", 1000
		p, err := l.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		slices.Reverse(p.Lines)
		return p.Lines
	}()
	if len(all) != len(records) {
		t.Fatalf("store holds %d, read %d", len(records), len(all))
	}
	seqs := map[int]string{}
	for _, line := range all {
		s := line.Seq
		if _, dup := seqs[s]; s == 0 || dup {
			t.Fatalf("seq %d of %q is not unique", s, line.Message)
		}
		seqs[s] = line.Message
	}
	page := func(q observe.LogQuery) observe.LogPage {
		t.Helper()
		p, err := l.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	messages := func(lines []observe.LogLine) []string {
		out := make([]string, len(lines))
		for i, line := range lines {
			out[i] = line.Message
		}
		return out
	}
	want := messages(all)

	for index, line := range all {
		anchor, ok := observe.ParseSeq(origin, strconv.Itoa(line.Seq))
		if !ok {
			t.Fatal("seq did not parse")
		}
		// Older from the seq: the page ends with the line (newest first).
		q := window()
		q.Direction, q.Anchor = "older", anchor
		before := page(q)
		if len(before.Lines) == 0 || before.Lines[0].Message != line.Message || before.Lines[0].Seq != line.Seq {
			t.Fatalf("older page from %q starts with %v", line.Message, messages(before.Lines))
		}
		// Walk both ways from it: every line exactly once, in order.
		var got []string // newest first while walking back
		for p := before; ; {
			got = append(got, messages(p.Lines)...)
			for _, x := range p.Lines {
				if seqs[x.Seq] != x.Message {
					t.Fatalf("paged seq %d of %q differs from the whole read", x.Seq, x.Message)
				}
			}
			if p.Older == "" {
				break
			}
			q := window()
			q.Direction, q.Cursor = "older", p.Older
			p = page(q)
		}
		got = reverse(got)
		if before.Newer == "" {
			t.Fatal("anchored older page carries no newer cursor")
		}
		for cursor := before.Newer; ; {
			q := window()
			q.Direction, q.Cursor = "newer", cursor
			p := page(q)
			if len(p.Lines) == 0 {
				break
			}
			got = append(got, reverse(messages(p.Lines))...)
			cursor = p.Newer
		}
		if !slices.Equal(got, want) {
			t.Fatalf("from %q (older):\n got %v\nwant %v", line.Message, got, want)
		}

		// Newer from the seq: the page starts with the line (oldest last
		// in the answer), its older cursor excludes it.
		q = window()
		q.Direction, q.Anchor = "newer", anchor
		after := page(q)
		if n := len(after.Lines); n == 0 || after.Lines[n-1].Message != line.Message {
			t.Fatalf("newer page from %q is %v", line.Message, messages(after.Lines))
		}
		q = window()
		q.Direction, q.Cursor = "older", after.Older
		prev := page(q)
		if index > 0 && (len(prev.Lines) == 0 || prev.Lines[0].Message != all[index-1].Message) {
			t.Fatalf("older cursor of the newer page from %q starts with %v", line.Message, messages(prev.Lines))
		}
		if index == 0 && len(prev.Lines) != 0 {
			t.Fatalf("older cursor of the first line returns %v", messages(prev.Lines))
		}
	}

	// A seq the selection does not hold still lands next to its microsecond.
	q := window()
	q.Direction, q.Anchor = "older", observe.LineAnchor{At: burst.Truncate(time.Microsecond), Bucket: 1<<10 - 1}
	for _, line := range all {
		if line.Seq&(1<<10-1) == int(q.Anchor.Bucket) {
			t.Skip("fixture holds the missing bucket")
		}
	}
	p := page(q)
	if len(p.Lines) == 0 || p.Lines[0].Message != "burst-late" {
		t.Fatalf("missing line: older page is %v", messages(p.Lines))
	}
}

func reverse(s []string) []string {
	out := slices.Clone(s)
	slices.Reverse(out)
	return out
}
