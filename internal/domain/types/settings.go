package types

import (
	"fmt"
	"net/url"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/webhook"
)

// YandexCloudSettings holds Yandex Cloud-specific credentials and resource IDs.
type YandexCloudSettings struct {
	// Credentials — passed as env vars to terraform.
	Token   string `json:"token"`    // YC_TOKEN (OAuth or IAM token)
	CloudID string `json:"cloud_id"` // YC_CLOUD_ID
	// Infrastructure.
	FolderID       string `json:"folder_id"`
	Zone           string `json:"zone"`
	NetworkID      string `json:"network_id"`   // external VPC network ID
	NetworkName    string `json:"network_name"` // subnet name prefix
	SubnetCIDR     string `json:"subnet_cidr"`
	PlatformID     string `json:"platform_id"` // e.g. standard-v2
	ImageID        string `json:"image_id"`
	AssignPublicIP bool   `json:"assign_public_ip"` // allocate external IP on VMs
	SSHUser        string `json:"ssh_user"`         // login user on VMs (default "stroppy")
	SSHPublicKey   string `json:"ssh_public_key"`
}

// Validate checks that all required fields for a Yandex Cloud run are set.
func (y YandexCloudSettings) Validate() error {
	missing := func(name, val string) error {
		if val == "" {
			return fmt.Errorf("yandex cloud: %s is required", name)
		}
		return nil
	}
	for _, check := range []struct{ name, val string }{
		{"token", y.Token},
		{"cloud_id", y.CloudID},
		{"folder_id", y.FolderID},
		{"network_id", y.NetworkID},
		{"network_name", y.NetworkName},
		{"subnet_cidr", y.SubnetCIDR},
		{"image_id", y.ImageID},
	} {
		if err := missing(check.name, check.val); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCloud checks that cloud-level settings required for any cloud provider are set.
func (c CloudSettings) ValidateCloud() error {
	if c.ServerAddr == "" {
		return fmt.Errorf("cloud: server_addr is required")
	}
	return nil
}

// CloudSettings holds cloud provider template settings used during provisioning.
type CloudSettings struct {
	Yandex     YandexCloudSettings `json:"yandex"`
	ServerAddr string              `json:"server_addr"` // external address for cloud-init callback
	BinaryURL  string              `json:"binary_url"`  // override; defaults to self-serve
}

// MonitoringStack describes the monitoring agents deployed on each machine.
type MonitoringStack struct {
	NodeExporterVersion     string `json:"node_exporter_version"`
	PostgresExporterVersion string `json:"postgres_exporter_version"`
	OtelColVersion          string `json:"otel_col_version"`
	VmagentVersion          string `json:"vmagent_version"`
	EtcdVersion             string `json:"etcd_version"`
}

// StroppySettings holds default stroppy configuration applied to every run.
// Fields map directly to K6_OTEL_* environment variables.
type StroppySettings struct {
	Version          string `json:"version"`
	OTLPExporterType string `json:"otlp_exporter_type"` // K6_OTEL_EXPORTER_TYPE (http|grpc)
	OTLPEndpoint     string `json:"otlp_endpoint"`      // K6_OTEL_HTTP_EXPORTER_ENDPOINT — set at runtime from monitoring URL
	OTLPURLPath      string `json:"otlp_url_path"`      // K6_OTEL_HTTP_EXPORTER_URL_PATH
	OTLPInsecure     bool   `json:"otlp_insecure"`      // K6_OTEL_HTTP_EXPORTER_INSECURE
	OTLPHeaders      string `json:"otlp_headers"`       // K6_OTEL_HEADERS (e.g. Authorization=Basic ...)
	OTLPMetricPrefix string `json:"otlp_metric_prefix"` // K6_OTEL_METRIC_PREFIX
	OTLPServiceName  string `json:"otlp_service_name"`  // K6_OTEL_SERVICE_NAME
}

// SetFromMonitoringURL configures OTLP settings from a monitoring URL (vmauth).
// K6 expects endpoint as host:port (no scheme), path separate, and token in headers.
func (s *StroppySettings) SetFromMonitoringURL(monitoringURL, token string, accountID int32) {
	u, err := url.Parse(monitoringURL)
	if err != nil {
		return
	}
	// K6_OTEL_HTTP_EXPORTER_ENDPOINT = host:port (no scheme)
	s.OTLPEndpoint = u.Host
	// K6_OTEL_HTTP_EXPORTER_URL_PATH = /insert/<accountID>/opentelemetry/v1/metrics
	s.OTLPURLPath = fmt.Sprintf("/insert/%d/opentelemetry/v1/metrics", accountID)
	// K6_OTEL_HTTP_EXPORTER_INSECURE = true for http, false for https
	s.OTLPInsecure = u.Scheme != "https"
	// Auth header
	if token != "" {
		s.OTLPHeaders = "Authorization=Bearer " + token
	}
}

// StroppyEnv returns the K6_OTEL_* environment variables for stroppy execution.
func (s StroppySettings) StroppyEnv(runID string) map[string]string {
	env := make(map[string]string)

	set := func(k, v string) {
		if v != "" {
			env[k] = v
		}
	}

	// NOTE: Do NOT set K6_OUT env var — it activates K6 "cli mode" which
	// overrides scenario-based execution in stroppy presets.
	// OTEL output is enabled via --out opentelemetry CLI flag instead.

	set("K6_OTEL_EXPORTER_TYPE", s.OTLPExporterType)
	set("K6_OTEL_HTTP_EXPORTER_ENDPOINT", s.OTLPEndpoint)
	set("K6_OTEL_HTTP_EXPORTER_URL_PATH", s.OTLPURLPath)
	set("K6_OTEL_HEADERS", s.OTLPHeaders)
	set("K6_OTEL_METRIC_PREFIX", s.OTLPMetricPrefix)
	set("K6_OTEL_SERVICE_NAME", s.OTLPServiceName)

	if s.OTLPInsecure {
		env["K6_OTEL_HTTP_EXPORTER_INSECURE"] = "true"
	}

	// Inject service name and run_id as OTEL resource attributes for correlation.
	env["OTEL_RESOURCE_ATTRIBUTES"] = fmt.Sprintf("service.name=stroppy,stroppy.run.id=%s", runID)

	return env
}

// ServerSettings is the per-tenant settings (stored in DB).
type ServerSettings struct {
	Cloud    CloudSettings  `json:"cloud"`
	Webhooks webhook.Config `json:"webhooks"`
}

// DefaultServerSettings returns ServerSettings populated with sensible defaults.
func DefaultServerSettings() ServerSettings {
	return ServerSettings{
		Cloud: CloudSettings{
			Yandex: YandexCloudSettings{
				Zone:        "ru-central1-b",
				NetworkName: "stroppy-net",
				SubnetCIDR:  "10.1.0.0/16",
				PlatformID:  "standard-v2",
			},
		},
	}
}

// DefaultMonitoring returns the platform-wide monitoring agent versions.
func DefaultMonitoring() MonitoringStack {
	return MonitoringStack{
		NodeExporterVersion:     "1.9.1",
		PostgresExporterVersion: "0.16.0",
		OtelColVersion:          "0.127.0",
		VmagentVersion:          "1.139.0",
		EtcdVersion:             "3.5.17",
	}
}

// DefaultStroppySettings returns platform-wide stroppy OTLP defaults.
func DefaultStroppySettings() StroppySettings {
	return StroppySettings{
		Version:          "4.1.0",
		OTLPExporterType: "http",
		OTLPInsecure:     true,
		OTLPURLPath:      "/opentelemetry/v1/metrics",
		OTLPMetricPrefix: "stroppy_",
		OTLPServiceName:  "stroppy",
	}
}
