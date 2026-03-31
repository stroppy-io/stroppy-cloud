package victoria

import (
	"encoding/json"
	"testing"
)

func TestQueryResult_Unmarshal(t *testing.T) {
	raw := `{
		"status": "success",
		"data": {
			"resultType": "matrix",
			"result": [
				{
					"metric": {"__name__": "cpu_usage", "instance": "db1:9090"},
					"values": [
						[1700000000, "0.42"],
						[1700000060, "0.55"]
					]
				}
			]
		}
	}`

	var qr QueryResult
	if err := json.Unmarshal([]byte(raw), &qr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if qr.Status != "success" {
		t.Errorf("expected status 'success', got %q", qr.Status)
	}
	if qr.Data.ResultType != "matrix" {
		t.Errorf("expected resultType 'matrix', got %q", qr.Data.ResultType)
	}
	if len(qr.Data.Result) != 1 {
		t.Fatalf("expected 1 series, got %d", len(qr.Data.Result))
	}

	series := qr.Data.Result[0]
	if series.Metric["__name__"] != "cpu_usage" {
		t.Errorf("expected metric name 'cpu_usage', got %q", series.Metric["__name__"])
	}
	if len(series.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(series.Values))
	}
}

func TestQueryResult_InstantQuery(t *testing.T) {
	raw := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"__name__": "up"},
					"value": [1700000000, "1"]
				}
			]
		}
	}`

	var qr QueryResult
	if err := json.Unmarshal([]byte(raw), &qr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if qr.Data.ResultType != "vector" {
		t.Errorf("expected resultType 'vector', got %q", qr.Data.ResultType)
	}
	if len(qr.Data.Result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(qr.Data.Result))
	}

	series := qr.Data.Result[0]
	if len(series.Value) != 2 {
		t.Fatalf("expected 2 elements in value, got %d", len(series.Value))
	}
}

func TestSamplePair_Timestamp(t *testing.T) {
	sp := SamplePair{float64(1700000000), "0.42"}

	ts := sp.Timestamp()
	if ts != 1700000000 {
		t.Errorf("expected timestamp 1700000000, got %f", ts)
	}
}

func TestSamplePair_Timestamp_NonFloat(t *testing.T) {
	sp := SamplePair{"not-a-number", "0.42"}

	ts := sp.Timestamp()
	if ts != 0 {
		t.Errorf("expected 0 for non-float timestamp, got %f", ts)
	}
}

func TestSamplePair_Val(t *testing.T) {
	sp := SamplePair{float64(1700000000), "0.42"}

	v := sp.Val()
	if v != "0.42" {
		t.Errorf("expected '0.42', got %q", v)
	}
}

func TestSamplePair_Val_NonString(t *testing.T) {
	sp := SamplePair{float64(1700000000), 42}

	v := sp.Val()
	if v != "" {
		t.Errorf("expected empty string for non-string value, got %q", v)
	}
}

func TestSamplePair_UnmarshalFromJSON(t *testing.T) {
	raw := `[1700000000, "0.55"]`
	var sp SamplePair
	if err := json.Unmarshal([]byte(raw), &sp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	ts := sp.Timestamp()
	if ts != 1700000000 {
		t.Errorf("expected timestamp 1700000000, got %f", ts)
	}

	v := sp.Val()
	if v != "0.55" {
		t.Errorf("expected '0.55', got %q", v)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8428")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.baseURL != "http://localhost:8428" {
		t.Errorf("unexpected baseURL: %s", c.baseURL)
	}
}

func TestQueryResult_EmptyResult(t *testing.T) {
	raw := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": []
		}
	}`

	var qr QueryResult
	if err := json.Unmarshal([]byte(raw), &qr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(qr.Data.Result) != 0 {
		t.Errorf("expected empty result, got %d", len(qr.Data.Result))
	}
}
