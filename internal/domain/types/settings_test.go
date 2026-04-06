package types

import (
	"testing"
)

func TestStroppyEnv_AllFieldsSet(t *testing.T) {
	s := StroppySettings{
		OTLPExporterType: "http",
		OTLPURLPath:      "/v1/metrics",
		OTLPInsecure:     true,
		OTLPHeaders:      "Authorization=Basic abc",
		OTLPMetricPrefix: "stroppy_",
		OTLPServiceName:  "stroppy-test",
	}

	env := s.StroppyEnv("run-123")

	expected := map[string]string{
		"K6_OTEL_EXPORTER_TYPE":          "http",
		"K6_OTEL_HTTP_EXPORTER_URL_PATH": "/v1/metrics",
		"K6_OTEL_HTTP_EXPORTER_INSECURE": "true",
		"K6_OTEL_HEADERS":                "Authorization=Basic abc",
		"K6_OTEL_METRIC_PREFIX":          "stroppy_",
		"K6_OTEL_SERVICE_NAME":           "stroppy-test",
		"OTEL_RESOURCE_ATTRIBUTES":       "service.name=stroppy,stroppy.run.id=run-123",
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

	// K6_OUT is NOT set (it breaks K6 scenarios). OTEL output is via --out CLI flag.
	if _, ok := env["K6_OUT"]; ok {
		t.Errorf("K6_OUT should not be set, got %q", env["K6_OUT"])
	}
	if env["OTEL_RESOURCE_ATTRIBUTES"] != "service.name=stroppy,stroppy.run.id=" {
		t.Errorf("expected OTEL_RESOURCE_ATTRIBUTES to be set, got %q", env["OTEL_RESOURCE_ATTRIBUTES"])
	}
}

func TestStroppyEnv_InsecureFalse(t *testing.T) {
	s := StroppySettings{
		OTLPInsecure: false,
	}
	env := s.StroppyEnv("")

	// When insecure is false, the key should not be set (only set when true).
	if _, ok := env["K6_OTEL_HTTP_EXPORTER_INSECURE"]; ok {
		t.Error("insecure key should not be set when insecure is false")
	}
}

func TestStroppyEnv_RunIDResourceAttribute(t *testing.T) {
	s := StroppySettings{}
	env := s.StroppyEnv("my-run")

	expected := "service.name=stroppy,stroppy.run.id=my-run"
	if env["OTEL_RESOURCE_ATTRIBUTES"] != expected {
		t.Errorf("expected resource attributes %q, got %q", expected, env["OTEL_RESOURCE_ATTRIBUTES"])
	}
}

func TestStroppyEnv_NoRunID(t *testing.T) {
	s := StroppySettings{
		OTLPExporterType: "http",
	}
	env := s.StroppyEnv("")

	// OTEL_RESOURCE_ATTRIBUTES is always set (with empty run id).
	if env["OTEL_RESOURCE_ATTRIBUTES"] != "service.name=stroppy,stroppy.run.id=" {
		t.Errorf("expected OTEL_RESOURCE_ATTRIBUTES with empty run id, got %q", env["OTEL_RESOURCE_ATTRIBUTES"])
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

func TestDefaultMonitoring(t *testing.T) {
	mon := DefaultMonitoring()

	if mon.NodeExporterVersion == "" {
		t.Error("NodeExporterVersion should not be empty")
	}
	if mon.PostgresExporterVersion == "" {
		t.Error("PostgresExporterVersion should not be empty")
	}
	if mon.OtelColVersion == "" {
		t.Error("OtelColVersion should not be empty")
	}
	if mon.VmagentVersion == "" {
		t.Error("VmagentVersion should not be empty")
	}
	if mon.EtcdVersion == "" {
		t.Error("EtcdVersion should not be empty")
	}
}

func TestDefaultStroppySettings(t *testing.T) {
	ss := DefaultStroppySettings()

	if ss.Version == "" {
		t.Error("stroppy version should not be empty")
	}
	if ss.OTLPExporterType != "http" {
		t.Errorf("expected OTLPExporterType=http, got %s", ss.OTLPExporterType)
	}
	if ss.OTLPMetricPrefix != "stroppy_" {
		t.Errorf("expected OTLPMetricPrefix=stroppy_, got %s", ss.OTLPMetricPrefix)
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
