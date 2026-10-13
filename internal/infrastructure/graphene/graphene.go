// Package graphene is the server's door to the Graphene installation:
// config, client construction and the operations the facade needs, scoped
// per tenant namespace. Transport is decided in client.go.
package graphene

import "time"

// Config is the `infra.graphene` section. The single service account
// (rolebinding namespace "*") authenticates every call; the tenant
// namespace travels per request.
type Config struct {
	// Address is the Graphene door, host:port.
	Address string `mapstructure:"address" validate:"required,hostname_port"`
	// Token is the service-account token (secret; env only).
	Token string `mapstructure:"token" validate:"required"`
	// Insecure switches to plain HTTP (dev installations only).
	Insecure bool `default:"false" mapstructure:"insecure"`
	// CallTimeout bounds one request-response call (retries included) when
	// the caller set no earlier deadline; streams are not bounded. A user
	// waits for these calls, so a hung Graphene fails them fast.
	CallTimeout time.Duration `default:"10s" mapstructure:"call_timeout"`
}

// NamespaceHeader carries the tenant namespace on every call.
const NamespaceHeader = "x-graphene-namespace"

// Namespace is the Graphene namespace of a tenant.
func Namespace(slug string) string { return "t-" + slug }
