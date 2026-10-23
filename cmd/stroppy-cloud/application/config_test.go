package application

import (
	"testing"
	"time"

	"github.com/gopherex/xconf/contrib/sources/env"
	"github.com/stretchr/testify/require"
)

func load(t *testing.T, vars ...string) *Config {
	t.Helper()
	t.Chdir(t.TempDir()) // no .env
	cfg, err := loadConfig(env.New(append(envOptions(), env.Environment(vars))...))
	require.NoError(t, err)
	return cfg
}

var required = []string{
	"STROPPY_INFRA_GRAPHENE_ADDRESS=graphene.stroppy.io:443",
	"STROPPY_INFRA_GRAPHENE_TOKEN=secret",
}

func TestConfigDefaults(t *testing.T) {
	cfg := load(t, required...)
	require.Equal(t, ":18347", cfg.HTTP.Addr)
	require.Equal(t, 2*time.Second, cfg.HTTP.WSPoll)
	require.Equal(t, []string{"https://stroppy.io"}, cfg.HTTP.PublicCORSOrigins)
	require.Equal(t, "localhost", cfg.Infra.Postgres.Host)
	require.Equal(t, 5432, cfg.Infra.Postgres.Port)
	require.Equal(t, "warn", cfg.Infra.Postgres.QueryLogLevel)
	require.Equal(t, 10*time.Second, cfg.Infra.Graphene.CallTimeout)
	require.Equal(t, "stroppy-cloud", cfg.Auth.Issuer)
	require.Equal(t, "http://localhost:4433", cfg.Auth.KratosUpstream)
	require.Equal(t, "starttls", cfg.Mail.TLS)
	require.Equal(t, 10*time.Minute, cfg.Infra.Pipelines.Interval)
	require.Equal(t, "docker.stroppy.io/stroppy-io/distroless-static:nonroot", cfg.Infra.Pipelines.BaseImage)
	require.True(t, cfg.Migrate)
	require.True(t, cfg.Trace.Enabled)
	require.False(t, cfg.Dev.Enabled())
}

func TestConfigFromEnvironment(t *testing.T) {
	cfg := load(t, append(required,
		"STROPPY_HTTP_PUBLIC_URL=https://cloud.stroppy.io",
		"STROPPY_HTTP_PUBLIC_CORS_ORIGINS=https://stroppy.io,https://www.stroppy.io",
		"STROPPY_ADMIN_EMAILS=a@stroppy.io,b@stroppy.io",
		"STROPPY_DEV_USERS=tok=dev@stroppy.local",
		"STROPPY_INFRA_POSTGRES_HOST=db",
		"STROPPY_INFRA_POSTGRES_POOL_MAX_CONNS=20",
		"STROPPY_INFRA_POSTGRES_POOL_MAX_CONN_LIFETIME=1h",
		"STROPPY_INFRA_GRAPHENE_INSECURE=true",
		"STROPPY_AUTH_ISSUER=iss",
		"STROPPY_AUTH_JWKS_FILE=/etc/jwks.json",
		"STROPPY_AUTH_KRATOS_PUBLIC_URL=https://cloud.stroppy.io/v1",
		"STROPPY_MIGRATE=false",
		"STROPPY_LOG_LEVEL=debug",
	)...)
	require.Equal(t, "https://cloud.stroppy.io", cfg.HTTP.PublicURL)
	require.Equal(t, []string{"https://stroppy.io", "https://www.stroppy.io"}, cfg.HTTP.PublicCORSOrigins)
	require.Equal(t, []string{"a@stroppy.io", "b@stroppy.io"}, cfg.AdminEmails)
	require.True(t, cfg.Dev.Enabled())
	require.Equal(t, "db", cfg.Infra.Postgres.Host)
	require.EqualValues(t, 20, cfg.Infra.Postgres.PoolMaxConns)
	require.Equal(t, time.Hour, cfg.Infra.Postgres.PoolMaxConnLifetime)
	require.True(t, cfg.Infra.Graphene.Insecure)
	require.Equal(t, "secret", cfg.Infra.Graphene.Token)
	require.Equal(t, "iss", cfg.Auth.Issuer)
	require.Equal(t, "/etc/jwks.json", cfg.Auth.JWKSFile)
	require.Equal(t, "https://cloud.stroppy.io/v1", cfg.Auth.KratosPublicURL)
	require.False(t, cfg.Migrate)
	require.Equal(t, "debug", cfg.Log.Level)
}

func TestConfigRefusesBadValues(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, bad := range [][]string{
		{"STROPPY_INFRA_GRAPHENE_TOKEN=secret"}, // no address
		append(required, "STROPPY_LOG_LEVEL=loud"),
		append(required, "STROPPY_INFRA_POSTGRES_PORT=70000"),
	} {
		_, err := loadConfig(env.New(append(envOptions(), env.Environment(bad))...))
		require.Error(t, err, bad)
	}
}
