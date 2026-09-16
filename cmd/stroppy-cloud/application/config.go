// Package application is the root of assembly: config, logger, observability,
// connections, composition of services, transport and shutdown. No domain
// logic — only "who is built from what".
package application

import (
	"time"

	"github.com/gopherex/xconf/pkg/structconf"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/mail"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/pipelines"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
)

/*
CONFIG: a section belongs to whoever owns the component — infra packages carry
their own. Field precedence: `default` tag < .env < environment variable. Env
name = path with underscores under the STROPPY_ prefix
(infra.postgres.host -> STROPPY_INFRA_POSTGRES_HOST).
*/

// EnvPrefix is the environment prefix of every setting.
const EnvPrefix = "STROPPY"

// Config is the whole process.
type Config struct {
	Log   LogConfig   `mapstructure:"log"`
	Trace TraceConfig `mapstructure:"trace"`
	HTTP  HTTPConfig  `mapstructure:"http"`
	IAM   IAMConfig   `mapstructure:"iam"`
	Mail  mail.Config `mapstructure:"mail"`
	Infra InfraConfig `mapstructure:"infra"`
	// AdminEmails are the bootstrap platform admins (IAM e-mails); the
	// database flag adds more from the admin UI.
	AdminEmails []string `mapstructure:"admin_emails"`
	// Migrate applies the schema on startup. The migrator takes an advisory
	// lock, so several replicas starting together are safe.
	Migrate bool `default:"true" mapstructure:"migrate"`
}

// LogConfig is the root logger.
type LogConfig struct {
	Level  string `default:"info" mapstructure:"level"  validate:"required,oneof=trace debug info warn error critical"`
	Format string `default:"json" mapstructure:"format" validate:"required,oneof=json console"`
}

// TraceConfig is telemetry. Endpoint, protocol and headers come from the
// standard OTEL_* environment; here only the switch.
type TraceConfig struct {
	Enabled bool `default:"true" mapstructure:"enabled"`
}

// HTTPConfig is the single public listener: API, SPA, IAM proxy, probes.
type HTTPConfig struct {
	Addr string `default:":8080" mapstructure:"addr" validate:"required"`
	// WSPoll is how often the WebSocket topics are refreshed from the
	// projection.
	WSPoll time.Duration `default:"2s" mapstructure:"ws_poll"`
	// PublicURL is the origin the SPA is served from; used for absolute
	// links (invites, share) and as the IAM proxy's cookie host.
	PublicURL string `default:"http://localhost:8080" mapstructure:"public_url" validate:"required,url"`
	// GrafanaURL is the installation's Grafana, relayed under /grafana.
	// Empty disables the relay.
	GrafanaURL string `mapstructure:"grafana_url" validate:"omitempty,url"`
	// GrafanaDashboards are the run-detail dashboards, `uid=Title[:per_machine]`.
	GrafanaDashboards []string `mapstructure:"grafana_dashboards"`
	// CORSOrigins are extra origins allowed to call the API (the Vite dev
	// server); the public URL's origin is always allowed.
	CORSOrigins []string `mapstructure:"cors_origins"`
}

// InfraConfig groups every external system under `infra.*`.
type InfraConfig struct {
	Postgres      postgres.Config     `mapstructure:"postgres"`
	Graphene      graphene.Config     `mapstructure:"graphene"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	// Pipelines is where the pipeline binaries shipped with the server
	// live (pushed into every tenant namespace, §7); empty = no push.
	Pipelines PipelinesConfig `mapstructure:"pipelines"`
	// Victoria is the telemetry stores (logs/metrics of runs).
	Victoria victoria.Config `mapstructure:"victoria"`
}

// PipelinesConfig is the pipeline push worker.
type PipelinesConfig struct {
	Dir string `default:"/opt/stroppy/pipelines" mapstructure:"dir"`
	// Interval re-checks every namespace (failures, restarts).
	Interval time.Duration `default:"10m" mapstructure:"interval"`
	// PushTimeout bounds one `push` subprocess.
	PushTimeout time.Duration `default:"10m" mapstructure:"push_timeout"`
	// Runner replaces the subprocess runner (tests); never configured.
	Runner pipelines.Runner `mapstructure:"-"`
}

// ObservabilityConfig is where the runs send telemetry (the agent-side
// OTLP collector of the installation); empty leaves runs without export.
type ObservabilityConfig struct {
	OTLPEndpoint string `mapstructure:"otlp_endpoint" validate:"omitempty,url"`
	// OTLPHeaders is the comma-separated key=value list sent with every
	// export (collector auth).
	OTLPHeaders string `mapstructure:"otlp_headers"`
}

// LoadConfig reads defaults, .env and the environment (environment wins).
func LoadConfig() (*Config, error) {
	return structconf.Load[Config](
		structconf.WithDotEnv(),
		structconf.WithEnvPrefix(EnvPrefix),
	)
}
