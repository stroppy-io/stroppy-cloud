package types

// YandexCloudSettings holds Yandex Cloud-specific credentials and resource IDs.
type YandexCloudSettings struct {
	FolderID         string `json:"folder_id"`
	Zone             string `json:"zone"`
	SubnetID         string `json:"subnet_id"`
	ServiceAccountID string `json:"service_account_id"`
	SSHPublicKey     string `json:"ssh_public_key"`
	ImageID          string `json:"image_id"`
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
	VictoriaMetricsURL      string `json:"victoria_metrics_url"`
	VictoriaMetricsUser     string `json:"victoria_metrics_user"`
	VictoriaMetricsPassword string `json:"victoria_metrics_password"`
	VictoriaLogsURL         string `json:"victoria_logs_url"`
}

// StroppySettings holds default stroppy configuration applied to every run.
// Fields map directly to K6_OTEL_* environment variables.
type StroppySettings struct {
	Version          string `json:"version"`
	OTLPExporterType string `json:"otlp_exporter_type"` // K6_OTEL_EXPORTER_TYPE (http|grpc)
	OTLPEndpoint     string `json:"otlp_endpoint"`      // K6_OTEL_HTTP_EXPORTER_ENDPOINT
	OTLPURLPath      string `json:"otlp_url_path"`      // K6_OTEL_HTTP_EXPORTER_URL_PATH
	OTLPInsecure     bool   `json:"otlp_insecure"`      // K6_OTEL_HTTP_EXPORTER_INSECURE
	OTLPHeaders      string `json:"otlp_headers"`       // K6_OTEL_HEADERS (e.g. Authorization=Basic ...)
	OTLPMetricPrefix string `json:"otlp_metric_prefix"` // K6_OTEL_METRIC_PREFIX
	OTLPServiceName  string `json:"otlp_service_name"`  // K6_OTEL_SERVICE_NAME
}

// StroppyEnv returns the K6_OTEL_* environment variables for stroppy execution.
func (s StroppySettings) StroppyEnv(runID string) map[string]string {
	env := make(map[string]string)

	set := func(k, v string) {
		if v != "" {
			env[k] = v
		}
	}

	// Enable k6 OTEL output extension.
	env["K6_OUT"] = "experimental-opentelemetry"

	set("K6_OTEL_EXPORTER_TYPE", s.OTLPExporterType)
	set("K6_OTEL_HTTP_EXPORTER_ENDPOINT", s.OTLPEndpoint)
	set("K6_OTEL_HTTP_EXPORTER_URL_PATH", s.OTLPURLPath)
	set("K6_OTEL_HEADERS", s.OTLPHeaders)
	set("K6_OTEL_METRIC_PREFIX", s.OTLPMetricPrefix)
	set("K6_OTEL_SERVICE_NAME", s.OTLPServiceName)

	if s.OTLPInsecure {
		env["K6_OTEL_HTTP_EXPORTER_INSECURE"] = "true"
	} else if s.OTLPEndpoint != "" {
		env["K6_OTEL_HTTP_EXPORTER_INSECURE"] = "false"
	}

	// Inject run_id as OTEL resource attribute for correlation.
	if runID != "" {
		env["K6_OTEL_RESOURCE_ATTRIBUTES"] = "stroppy_run_id=" + runID
	}

	return env
}

// GrafanaSettings configures the Grafana integration for embedded dashboards.
type GrafanaSettings struct {
	URL          string            `json:"url"`           // e.g. "http://localhost:3001"
	EmbedEnabled bool              `json:"embed_enabled"` // whether to show embedded dashboards
	Dashboards   map[string]string `json:"dashboards"`    // name -> uid
}

// ServerSettings is the top-level admin settings struct combining all subsections.
type ServerSettings struct {
	Cloud           CloudSettings   `json:"cloud"`
	Monitoring      MonitoringStack `json:"monitoring"`
	Packages        PackageDefaults `json:"packages"`
	StroppyDefaults StroppySettings `json:"stroppy_defaults"`
	Grafana         GrafanaSettings `json:"grafana"`
}

// DefaultServerSettings returns ServerSettings populated with sensible defaults.
func DefaultServerSettings() ServerSettings {
	return ServerSettings{
		Cloud: CloudSettings{
			Yandex: YandexCloudSettings{
				Zone: "ru-central1-b",
			},
		},
		Monitoring: MonitoringStack{
			NodeExporterVersion:     "1.9.1",
			PostgresExporterVersion: "0.16.0",
			OtelColVersion:          "0.127.0",
			VmagentVersion:          "1.115.0",
			EtcdVersion:             "3.5.17",
		},
		Packages: DefaultPackages(),
		StroppyDefaults: StroppySettings{
			Version:          "3.1.0",
			OTLPExporterType: "http",
			OTLPEndpoint:     "172.17.0.1:8428",
			OTLPInsecure:     true,
			OTLPURLPath:      "/opentelemetry/v1/metrics",
			OTLPMetricPrefix: "stroppy_",
			OTLPServiceName:  "stroppy",
		},
		Grafana: GrafanaSettings{
			URL:          "http://localhost:3001",
			EmbedEnabled: true,
			Dashboards: map[string]string{
				"overview": "stroppy-run",
				"system":   "stroppy-system",
				"postgres": "stroppy-postgres",
				"mysql":    "stroppy-mysql",
				"picodata": "stroppy-picodata",
				"stroppy":  "stroppy-metrics-v1",
				"compare":  "stroppy-compare",
			},
		},
	}
}
