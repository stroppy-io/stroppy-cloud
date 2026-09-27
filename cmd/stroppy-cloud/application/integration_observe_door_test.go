//go:build integration

package application

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
)

// Only the fake Graphene installation knows the test stores' addresses.
// The application exercises the same Observe RPCs as a real installation.
func observeScope(header http.Header, ref string) (string, string, error) {
	ns := header.Get(graphene.NamespaceHeader)
	id := strings.TrimPrefix(ref, "run/")
	if ns == "" || id == "" || id == ref {
		return "", "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run ref and namespace required"))
	}
	return ns, id, nil
}

func observePost(ctx context.Context, address string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 != 2 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("test telemetry backend: %s", body))
	}
	return body, nil
}

func doorLogQuery(header http.Header, q *managementv1.LogsRequest) (string, error) {
	ns, id, err := observeScope(header, q.GetRef())
	if err != nil {
		return "", err
	}
	parts := []string{fmt.Sprintf(`"graphene.namespace":=%q AND "graphene.run":=%q`, ns, id)}
	if q.GetQuery() != "" {
		parts = append(parts, "("+q.GetQuery()+")")
	}
	if q.GetSinceUnixNano() != 0 {
		parts = append(parts, "_time:>"+time.Unix(0, q.GetSinceUnixNano()).UTC().Format(time.RFC3339Nano))
	}
	if q.GetUntilUnixNano() != 0 {
		parts = append(parts, "_time:<"+time.Unix(0, q.GetUntilUnixNano()).UTC().Format(time.RFC3339Nano))
	}
	if q.GetText() != "" {
		parts = append(parts, "_msg:~"+strconv.Quote("(?i)"+regexp.QuoteMeta(q.GetText())))
	}
	if len(q.GetSeverities()) > 0 {
		var levels []string
		for _, s := range q.GetSeverities() {
			levels = append(levels, strconv.Quote(strings.ToUpper(s)))
		}
		parts = append(parts, "severity_text:in("+strings.Join(levels, ",")+")")
	}
	return strings.Join(parts, " AND "), nil
}

func (f *fakeGraphene) Logs(ctx context.Context, req *connect.Request[managementv1.LogsRequest], stream *connect.ServerStream[managementv1.LogChunk]) error {
	q, err := doorLogQuery(req.Header(), req.Msg)
	if err != nil {
		return err
	}
	offset := 0
	if p := req.Msg.GetPageToken(); p != "" {
		offset, err = strconv.Atoi(strings.TrimPrefix(p, "test-page-"))
		if err != nil || offset < 0 {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid test page"))
		}
	}
	order := ""
	if req.Msg.GetOrder() == "desc" {
		order = " desc"
	}
	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 1000
	}
	q += fmt.Sprintf(" | sort by (_time, _stream_id, _msg)%s | offset %d | limit %d", order, offset, limit+1)
	body, err := observePost(ctx, victoriaLogsURL+"/select/logsql/query", url.Values{"query": {q}})
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(strings.NewReader(string(body)))
	count := 0
	for sc.Scan() {
		if count == limit {
			break
		}
		var fields map[string]string
		if err := json.Unmarshal(sc.Bytes(), &fields); err != nil {
			return err
		}
		at, err := time.Parse(time.RFC3339Nano, fields["_time"])
		if err != nil {
			return err
		}
		rec := &managementv1.LogRecord{TimeUnixNano: at.UnixNano(), Body: fields["_msg"], Severity: fields["severity_text"], Attributes: fields}
		if err := stream.Send(&managementv1.LogChunk{Chunk: &managementv1.LogChunk_Record{Record: rec}}); err != nil {
			return err
		}
		count++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	page := &managementv1.LogPage{Returned: int32(count)} //nolint:gosec // bounded by request
	if count == limit && len(sc.Bytes()) > 0 {
		page.Truncated = true
		page.NextPageToken = fmt.Sprintf("test-page-%d", offset+count)
	}
	return stream.Send(&managementv1.LogChunk{Chunk: &managementv1.LogChunk_Page{Page: page}})
}

func (f *fakeGraphene) LogFacets(ctx context.Context, req *connect.Request[managementv1.LogFacetsRequest]) (*connect.Response[managementv1.LogFacetsResponse], error) {
	q, err := doorLogQuery(req.Header(), req.Msg.GetSelection())
	if err != nil {
		return nil, err
	}
	out := &managementv1.LogFacetsResponse{}
	for _, field := range req.Msg.GetFields() {
		body, err := observePost(ctx, victoriaLogsURL+"/select/logsql/field_values", url.Values{"query": {q}, "field": {field}, "limit": {"50"}})
		if err != nil {
			return nil, err
		}
		var data struct {
			Values []struct {
				Value string `json:"value"`
				Hits  int64  `json:"hits"`
			} `json:"values"`
		}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}
		facet := &managementv1.LogFacetsResponse_Facet{Field: field}
		for _, v := range data.Values {
			facet.Values = append(facet.Values, &managementv1.LogFacetsResponse_Value{Value: v.Value, Hits: v.Hits})
		}
		out.Facets = append(out.Facets, facet)
	}
	return connect.NewResponse(out), nil
}

func (f *fakeGraphene) Metrics(ctx context.Context, req *connect.Request[managementv1.MetricsRequest], stream *connect.ServerStream[managementv1.MetricsChunk]) error {
	ns, id, err := observeScope(req.Header(), req.Msg.GetRef())
	if err != nil {
		return err
	}
	filters := []string{fmt.Sprintf(`{"graphene.namespace"=%q,"graphene.run"=%q}`, ns, id), fmt.Sprintf(`{graphene_namespace=%q,graphene_run=%q}`, ns, id)}
	step := max(req.Msg.GetStepSeconds(), 1)
	body, err := observePost(ctx, victoriaMetricsURL+"/api/v1/query_range", url.Values{
		"query": {req.Msg.GetQuery()}, "start": {strconv.FormatInt(req.Msg.GetStartUnixNano()/int64(time.Second), 10)}, "end": {strconv.FormatInt(req.Msg.GetEndUnixNano()/int64(time.Second), 10)}, "step": {strconv.Itoa(int(step))}, "extra_filters[]": filters,
	})
	if err != nil {
		return err
	}
	return stream.Send(&managementv1.MetricsChunk{Chunk: &managementv1.MetricsChunk_Snapshot{Snapshot: body}})
}
