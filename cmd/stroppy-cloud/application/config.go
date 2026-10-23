// Package application is the root of assembly: config, logger, observability,
// connections, composition of services, transport and shutdown. No domain
// logic — only "who is built from what".
package application

import (
	"context"
	"time"

	sp "github.com/gopherex/schemapb/go/schemapb"
	"github.com/gopherex/xconf"
	"github.com/gopherex/xconf/contrib/sources/dotenv"
	"github.com/gopherex/xconf/contrib/sources/env"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/mail"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/pipelines"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

/*
CONFIG: a section belongs to whoever owns the component — infra packages carry
their own. Field precedence: schemapb default < .env < environment variable.
Env name = path with underscores under the STROPPY_ prefix
(infra.postgres.host -> STROPPY_INFRA_POSTGRES_HOST); a list is one variable,
comma-separated.
*/

// EnvPrefix is the environment prefix of every setting.
const EnvPrefix = "STROPPY"

// Config is the whole process.
type Config struct {
	Log   LogConfig   `json:"log" schemapb:"default={}"`
	Trace TraceConfig `json:"trace" schemapb:"default={}"`
	HTTP  HTTPConfig  `json:"http" schemapb:"default={}"`
	Auth  AuthConfig  `json:"auth" schemapb:"default={}"`
	// Dev turns a local installation's login into static tokens (no Kratos).
	Dev   DevConfig   `json:"dev" schemapb:"default={}"`
	Mail  mail.Config `json:"mail" schemapb:"default={}"`
	Infra InfraConfig `json:"infra" schemapb:"default={}"`
	// AdminEmails are the bootstrap platform admins (verified Kratos
	// e-mails); the database flag adds more from the admin UI.
	AdminEmails []string `json:"admin_emails,omitempty"`
	// DemoReportURL is a public report (`<public url>/s/<token>`) of a real
	// run that first-run screens offer as an example result; empty = none.
	DemoReportURL string `json:"demo_report_url,omitempty" validate:"url"`
	// Migrate applies the schema on startup. The migrator takes an advisory
	// lock, so several replicas starting together are safe.
	Migrate bool `json:"migrate" schemapb:"default=true"`
}

// LogConfig is the root logger.
type LogConfig struct {
	Level  string `json:"level" schemapb:"default=info;in=[\"trace\",\"debug\",\"info\",\"warn\",\"error\",\"critical\"]"`
	Format string `json:"format" schemapb:"default=json;in=[\"json\",\"console\"]"`
}

// TraceConfig is telemetry. Endpoint, protocol and headers come from the
// standard OTEL_* environment; here only the switch.
type TraceConfig struct {
	Enabled bool `json:"enabled" schemapb:"default=true"`
}

// HTTPConfig is the single public listener: API, SPA, IAM proxy, probes.
type HTTPConfig struct {
	Addr string `json:"addr" schemapb:"default=:18347"`
	// InternalAddr is the listener for the cluster only, never routed
	// outside: the Kratos mail templates (courier.templates.* URLs).
	InternalAddr string `json:"internal_addr" schemapb:"default=:18348"`
	// WSPoll is how often the WebSocket topics are refreshed from the
	// projection.
	WSPoll time.Duration `json:"ws_poll" schemapb:"default=2s"`
	// PublicURL is the origin the SPA is served from; used for absolute
	// links (invites, share) and as the IAM proxy's cookie host.
	PublicURL string `json:"public_url" schemapb:"default=http://localhost:18347" validate:"url"`
	// CORSOrigins are extra origins allowed to call the API (the Vite dev
	// server); the public URL's origin is always allowed.
	CORSOrigins []string `json:"cors_origins,omitempty"`
	// PublicCORSOrigins may read the public API (/api/v1/public/*: rating,
	// shared reports) from a browser — the product site — and talk to the
	// embeds by postMessage. GET only, no credentials; the rest of the API
	// stays closed to them. Unset = defaultPublicCORSOrigins.
	PublicCORSOrigins []string `json:"public_cors_origins,omitempty"`
}

// InfraConfig groups every external system under `infra.*`.
type InfraConfig struct {
	Postgres postgres.Config `json:"postgres" schemapb:"default={}"`
	Graphene graphene.Config `json:"graphene" schemapb:"default={}"`
	// Pipelines is where the pipeline binaries shipped with the server
	// live (pushed into every tenant namespace, §7); empty = no push.
	Pipelines PipelinesConfig `json:"pipelines" schemapb:"default={}"`
	// RegistryMirror is a registry host proxying Docker Hub, GHCR and
	// Quay (for example docker.stroppy.io); run machines pull public
	// images through it. Empty = pull from the registries directly.
	RegistryMirror string `json:"registry_mirror,omitempty"`
}

// PipelinesConfig is the pipeline push worker.
type PipelinesConfig struct {
	Dir string `json:"dir" schemapb:"default=/opt/stroppy/pipelines"`
	// Interval re-checks every namespace (failures, restarts).
	Interval time.Duration `json:"interval" schemapb:"default=10m"`
	// BaseImage is what a pipeline image is built on: our registry's copy
	// of gcr.io/distroless/static:nonroot, since the cluster cannot count on
	// reaching gcr.io. Empty = Graphene's default, straight from gcr.io.
	BaseImage string `json:"base_image" schemapb:"default=docker.stroppy.io/stroppy-io/distroless-static:nonroot"`
	// PushTimeout bounds one `push` subprocess.
	PushTimeout time.Duration `json:"push_timeout" schemapb:"default=10m"`
	// Runner replaces the subprocess runner (tests); never configured.
	Runner pipelines.Runner `json:"-"`
}

// defaultPublicCORSOrigins stands in for an unset public CORS list (list
// defaults are not schema defaults).
var defaultPublicCORSOrigins = []string{"https://stroppy.io"}

// listEnv are the list settings, given as comma-separated variables.
var listEnv = []string{
	EnvPrefix + "_ADMIN_EMAILS",
	EnvPrefix + "_DEV_USERS",
	EnvPrefix + "_HTTP_CORS_ORIGINS",
	EnvPrefix + "_HTTP_PUBLIC_CORS_ORIGINS",
}

// LoadConfig reads defaults, .env and the environment (environment wins).
func LoadConfig() (*Config, error) {
	return loadConfig(env.New(envOptions()...))
}

func envOptions() []env.Option {
	opts := []env.Option{env.Prefix(EnvPrefix + "_")}
	for _, name := range listEnv {
		opts = append(opts, env.Split(name, ","))
	}
	return opts
}

func loadConfig(environment xconf.Source) (*Config, error) {
	schema, err := sp.ReflectType[Config](sp.ID("stroppy", "server.config", sp.Ver(1, 0, 0)))
	if err != nil {
		return nil, err
	}
	cfg, err := xconf.LoadAs[Config](context.Background(), schema,
		xconf.Optional(dotenv.File(".env", envOptions()...)),
		environment,
	)
	if err != nil {
		return nil, err
	}
	if cfg.HTTP.PublicCORSOrigins == nil {
		cfg.HTTP.PublicCORSOrigins = defaultPublicCORSOrigins
	}
	return &cfg, nil
}
