package types

import (
	"testing"
)

func TestStroppyEnv_AllFieldsSet(t *testing.T) {
	s := StroppySettings{
		OTLPExporterType: "http",
		OTLPEndpoint:     "https://metrics.example.com",
		OTLPURLPath:      "/v1/metrics",
		OTLPInsecure:     true,
		OTLPHeaders:      "Authorization=Basic abc",
		OTLPMetricPrefix: "stroppy_",
		OTLPServiceName:  "stroppy-test",
	}

	env := s.StroppyEnv("run-123")

	expected := map[string]string{
		"K6_OTEL_EXPORTER_TYPE":          "http",
		"K6_OTEL_HTTP_EXPORTER_ENDPOINT": "https://metrics.example.com",
		"K6_OTEL_HTTP_EXPORTER_URL_PATH": "/v1/metrics",
		"K6_OTEL_HTTP_EXPORTER_INSECURE": "true",
		"K6_OTEL_HEADERS":                "Authorization=Basic abc",
		"K6_OTEL_METRIC_PREFIX":          "stroppy_",
		"K6_OTEL_SERVICE_NAME":           "stroppy-test",
		"K6_OTEL_RESOURCE_ATTRIBUTES":    "stroppy_run_id=run-123",
	}

	for k, v := range expected {
		if env[k] != v {
			t.Errorf("env[%s] = %q, want %q", k, env[k], v)
		}
	}
}

func TestStroppyEnv_EmptyFields(t *testing.T) {
	s := StroppySettings{}
	env := s.StroppyEnv("")

	// With all empty fields and empty runID, env should be empty.
	if len(env) != 0 {
		t.Errorf("expected empty env map, got %v", env)
	}
}

func TestStroppyEnv_InsecureFalseWithEndpoint(t *testing.T) {
	s := StroppySettings{
		OTLPEndpoint: "https://metrics.example.com",
		OTLPInsecure: false,
	}
	env := s.StroppyEnv("")

	if env["K6_OTEL_HTTP_EXPORTER_INSECURE"] != "false" {
		t.Errorf("expected insecure=false when endpoint is set, got %q", env["K6_OTEL_HTTP_EXPORTER_INSECURE"])
	}
}

func TestStroppyEnv_InsecureFalseWithoutEndpoint(t *testing.T) {
	s := StroppySettings{
		OTLPInsecure: false,
	}
	env := s.StroppyEnv("")

	// When endpoint is empty and insecure is false, the key should not be set.
	if _, ok := env["K6_OTEL_HTTP_EXPORTER_INSECURE"]; ok {
		t.Error("insecure key should not be set when endpoint is empty and insecure is false")
	}
}

func TestStroppyEnv_RunIDResourceAttribute(t *testing.T) {
	s := StroppySettings{}
	env := s.StroppyEnv("my-run")

	expected := "stroppy_run_id=my-run"
	if env["K6_OTEL_RESOURCE_ATTRIBUTES"] != expected {
		t.Errorf("expected resource attributes %q, got %q", expected, env["K6_OTEL_RESOURCE_ATTRIBUTES"])
	}
}

func TestStroppyEnv_NoRunID(t *testing.T) {
	s := StroppySettings{
		OTLPExporterType: "http",
	}
	env := s.StroppyEnv("")

	if _, ok := env["K6_OTEL_RESOURCE_ATTRIBUTES"]; ok {
		t.Error("resource attributes should not be set when runID is empty")
	}
}

func TestDefaultServerSettings_Populated(t *testing.T) {
	ss := DefaultServerSettings()

	if ss.Cloud.Yandex.Zone == "" {
		t.Error("expected Yandex zone to be set")
	}
	if ss.Cloud.Yandex.Zone != "ru-central1-b" {
		t.Errorf("expected zone ru-central1-b, got %s", ss.Cloud.Yandex.Zone)
	}
}

func TestDefaultServerSettings_Monitoring(t *testing.T) {
	ss := DefaultServerSettings()

	if ss.Monitoring.NodeExporterVersion == "" {
		t.Error("NodeExporterVersion should not be empty")
	}
	if ss.Monitoring.PostgresExporterVersion == "" {
		t.Error("PostgresExporterVersion should not be empty")
	}
	if ss.Monitoring.OtelColVersion == "" {
		t.Error("OtelColVersion should not be empty")
	}
	if ss.Monitoring.VmagentVersion == "" {
		t.Error("VmagentVersion should not be empty")
	}
}

func TestDefaultServerSettings_StroppyDefaults(t *testing.T) {
	ss := DefaultServerSettings()

	if ss.StroppyDefaults.Version == "" {
		t.Error("stroppy version should not be empty")
	}
	if ss.StroppyDefaults.OTLPExporterType != "http" {
		t.Errorf("expected OTLPExporterType=http, got %s", ss.StroppyDefaults.OTLPExporterType)
	}
	if ss.StroppyDefaults.OTLPMetricPrefix != "stroppy_" {
		t.Errorf("expected OTLPMetricPrefix=stroppy_, got %s", ss.StroppyDefaults.OTLPMetricPrefix)
	}
}

func TestDefaultServerSettings_PackagesIncluded(t *testing.T) {
	ss := DefaultServerSettings()

	if len(ss.Packages.Postgres) == 0 {
		t.Error("packages should include postgres defaults")
	}
	if len(ss.Packages.MySQL) == 0 {
		t.Error("packages should include mysql defaults")
	}
	if len(ss.Packages.Picodata) == 0 {
		t.Error("packages should include picodata defaults")
	}
}
