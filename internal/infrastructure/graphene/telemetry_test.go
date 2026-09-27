package graphene

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
	"github.com/graphene-ci/graphene/pkg/proto/management/v1/managementv1connect"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
)

type observeDoor struct {
	managementv1connect.UnimplementedObserveAPIHandler
	logs    func(context.Context, *connect.Request[managementv1.LogsRequest], *connect.ServerStream[managementv1.LogChunk]) error
	metrics func(context.Context, *connect.Request[managementv1.MetricsRequest], *connect.ServerStream[managementv1.MetricsChunk]) error
}

func (d observeDoor) Logs(ctx context.Context, r *connect.Request[managementv1.LogsRequest], s *connect.ServerStream[managementv1.LogChunk]) error {
	return d.logs(ctx, r, s)
}
func (d observeDoor) Metrics(ctx context.Context, r *connect.Request[managementv1.MetricsRequest], s *connect.ServerStream[managementv1.MetricsChunk]) error {
	return d.metrics(ctx, r, s)
}

func observeClient(t *testing.T, d observeDoor) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(managementv1connect.NewObserveAPIHandler(d))
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return New(&Config{Address: s.Listener.Addr().String(), Insecure: true, Token: "test-token"})
}

func TestLogsPageAndTailPreserveEqualTimestamps(t *testing.T) {
	at := time.Now().UTC()
	count := 7
	scope := observe.Scope{Namespace: "tenant-a", Run: "run-a"}
	c := observeClient(t, observeDoor{logs: func(_ context.Context, r *connect.Request[managementv1.LogsRequest], s *connect.ServerStream[managementv1.LogChunk]) error {
		if r.Header().Get(NamespaceHeader) != scope.Namespace || r.Header().Get("Authorization") != "Bearer test-token" || r.Msg.GetRef() != "run/run-a" {
			t.Error("missing scoped authentication")
		}
		start := 0
		if r.Msg.GetPageToken() != "" {
			var err error
			start, err = strconv.Atoi(r.Msg.GetPageToken())
			if err != nil {
				return err
			}
		}
		end := min(start+int(r.Msg.GetLimit()), count)
		for i := start; i < end; i++ {
			n := i
			if r.Msg.GetOrder() == "desc" {
				n = count - i - 1
			}
			if err := s.Send(&managementv1.LogChunk{Chunk: &managementv1.LogChunk_Record{Record: &managementv1.LogRecord{TimeUnixNano: at.UnixNano(), Body: strconv.Itoa(n), Severity: "INFO"}}}); err != nil {
				return err
			}
		}
		page := &managementv1.LogPage{Returned: int32(end - start), Truncated: end < count} //nolint:gosec // tiny fixture
		if page.Truncated {
			page.NextPageToken = strconv.Itoa(end)
		}
		return s.Send(&managementv1.LogChunk{Chunk: &managementv1.LogChunk_Page{Page: page}})
	}})
	l := NewLogs(c)
	for _, direction := range []string{"older", "newer"} {
		q := observe.LogQuery{Scope: scope, Start: at.Add(-time.Minute), End: at.Add(time.Minute), Limit: 3, Direction: direction}
		var got []string
		for i := 0; i < 3; i++ {
			p, err := l.Query(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			lines := p.Lines
			if direction == "newer" {
				slices.Reverse(lines)
				q.Cursor = p.Newer
			} else {
				q.Cursor = p.Older
			}
			for _, r := range lines {
				got = append(got, r.Message)
			}
		}
		want := []string{"0", "1", "2", "3", "4", "5", "6"}
		if direction == "older" {
			slices.Reverse(want)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("%s: %v", direction, got)
		}
		if direction == "newer" {
			p, err := l.Query(context.Background(), q)
			if err != nil || len(p.Lines) != 0 {
				t.Fatalf("replayed delivered records: %+v %v", p, err)
			}
			count++
			p, err = l.Query(context.Background(), q)
			if err != nil || len(p.Lines) != 1 || p.Lines[0].Message != "7" {
				t.Fatalf("lost newly arrived equal timestamp: %+v %v", p, err)
			}
			q.Scope.Run = "other"
			if _, err = l.Query(context.Background(), q); err == nil {
				t.Fatal("accepted another run's cursor")
			}
		}
	}
}

func TestMetricsPassExpressionAndScopeToGraphene(t *testing.T) {
	const expr = `sum(rate(stroppy_iterations_total[1m])) or up{job="other"}`
	c := observeClient(t, observeDoor{metrics: func(_ context.Context, r *connect.Request[managementv1.MetricsRequest], s *connect.ServerStream[managementv1.MetricsChunk]) error {
		if r.Header().Get(NamespaceHeader) != "tenant-a" || r.Msg.GetRef() != "run/run-a" || r.Msg.GetQuery() != expr || r.Msg.GetStepSeconds() != 5 {
			t.Errorf("wrong query: %v", r.Msg)
		}
		return s.Send(&managementv1.MetricsChunk{Chunk: &managementv1.MetricsChunk_Snapshot{Snapshot: []byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`)}})
	}})
	_, err := NewMetrics(c).Raw(context.Background(), observe.Scope{Namespace: "tenant-a", Run: "run-a"}, expr, time.Now().Add(-time.Minute), time.Now(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewMetrics(c).Raw(context.Background(), observe.Scope{}, expr, time.Now(), time.Now(), 0); err == nil {
		t.Fatal("global query permitted")
	}
}

func TestLogsPropagateStreamFailure(t *testing.T) {
	c := observeClient(t, observeDoor{logs: func(context.Context, *connect.Request[managementv1.LogsRequest], *connect.ServerStream[managementv1.LogChunk]) error {
		return connect.NewError(connect.CodeUnavailable, errors.New("history unavailable"))
	}})
	_, err := NewLogs(c).Query(context.Background(), observe.LogQuery{Scope: observe.Scope{Namespace: "n", Run: "r"}, Start: time.Now().Add(-time.Minute), End: time.Now()})
	if err == nil {
		t.Fatal("failed history became an empty success")
	}
}
