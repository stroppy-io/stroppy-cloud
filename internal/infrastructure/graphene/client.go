package graphene

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/gopherex/xprobe"
	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
	"github.com/graphene-ci/graphene/pkg/proto/management/v1/managementv1connect"
)

/*
The Graphene door is one address; the management plane is its Connect half
(cmux hands raw gRPC to the Temporal proxy, so Connect is the only protocol
that reaches ResourcesAPI & co.). One service-account token authenticates
every call; the tenant namespace rides per request in a header, taken from
the context by the interceptor — callers scope with WithNamespace and never
touch headers.
*/

// Client bundles the management clients over one HTTP client.
type Client struct {
	Runs      managementv1connect.RunsAPIClient
	Resources managementv1connect.ResourcesAPIClient
	Observe   managementv1connect.ObserveAPIClient
	Secrets   managementv1connect.SecretsAPIClient
	Revisions managementv1connect.RevisionsAPIClient
	Source    managementv1connect.SourceAPIClient
	Rbac      managementv1connect.RbacAPIClient
	Ns        managementv1connect.NamespacesAPIClient
	Agents    managementv1connect.AgentsAPIClient
}

// New builds the clients. Nothing is dialed until the first call; Probe is
// the startup check.
func New(cfg *Config) *Client {
	scheme := "https"
	if cfg.Insecure {
		scheme = "http"
	}
	base := scheme + "://" + cfg.Address
	hc := &http.Client{}
	auth := connect.WithInterceptors(authInterceptor{token: cfg.Token, timeout: cfg.CallTimeout})
	return &Client{
		Runs:      managementv1connect.NewRunsAPIClient(hc, base, auth),
		Resources: managementv1connect.NewResourcesAPIClient(hc, base, auth),
		Observe:   managementv1connect.NewObserveAPIClient(hc, base, auth),
		Secrets:   managementv1connect.NewSecretsAPIClient(hc, base, auth),
		Revisions: managementv1connect.NewRevisionsAPIClient(hc, base, auth),
		Source:    managementv1connect.NewSourceAPIClient(hc, base, auth),
		Rbac:      managementv1connect.NewRbacAPIClient(hc, base, auth),
		Ns:        managementv1connect.NewNamespacesAPIClient(hc, base, auth),
		Agents:    managementv1connect.NewAgentsAPIClient(hc, base, auth),
	}
}

type nsKey struct{}

// WithNamespace scopes every Graphene call made with ctx to the tenant.
func WithNamespace(ctx context.Context, namespace string) context.Context {
	return context.WithValue(ctx, nsKey{}, namespace)
}

// NamespaceFrom returns the namespace of ctx ("" = the token's own).
func NamespaceFrom(ctx context.Context) string {
	ns, _ := ctx.Value(nsKey{}).(string) //nolint:errcheck // absent = token's namespace
	return ns
}

// authInterceptor stamps the token and the namespace of the context onto
// every call. Unary calls retry on Unavailable — a redeploy mid-request
// otherwise surfaces as a one-off EOF; streams re-establish themselves.
type authInterceptor struct {
	token   string
	timeout time.Duration
}

func (a authInterceptor) apply(ctx context.Context, h http.Header) {
	h.Set("Authorization", "Bearer "+a.token)
	if ns := NamespaceFrom(ctx); ns != "" {
		h.Set(NamespaceHeader, ns)
	}
}

func (a authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		a.apply(ctx, req.Header())
		if _, has := ctx.Deadline(); !has && a.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, a.timeout)
			defer cancel()
		}
		var res connect.AnyResponse
		var err error
		for attempt, backoff := 0, 300*time.Millisecond; ; attempt, backoff = attempt+1, backoff*2 {
			res, err = next(ctx, req)
			if err == nil || attempt == 2 || connect.CodeOf(err) != connect.CodeUnavailable {
				return res, err
			}
			select {
			case <-ctx.Done():
				return res, err
			case <-time.After(backoff):
			}
		}
	}
}

func (a authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		a.apply(ctx, conn.RequestHeader())
		return conn
	}
}

func (authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// Probe reports the door answering with its service account recognized.
func (c *Client) Probe() xprobe.Probe {
	return xprobe.FromError(func(ctx context.Context) error {
		_, err := c.Rbac.WhoAmI(ctx, connect.NewRequest(&managementv1.WhoAmIRequest{}))
		return err
	})
}

// --- tenant lifecycle -------------------------------------------------------

// NamespaceSpec is the nsflow record spec.
type NamespaceSpec struct {
	RetentionDays int32  `json:"retentionDays,omitempty"`
	Description   string `json:"description,omitempty"`
}

// EnsureNamespace applies the tenant namespace (idempotent). System kinds
// route to graphene-system server-side, so no namespace scoping here.
func (c *Client) EnsureNamespace(ctx context.Context, name string, spec NamespaceSpec, labels map[string]string) error {
	raw, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	_, err = c.Resources.Apply(ctx, connect.NewRequest(&managementv1.ApplyRequest{
		Kind: "namespace", Id: name, Spec: raw, Labels: labels,
	}))
	if err != nil {
		return fmt.Errorf("graphene: apply namespace %s: %w", name, err)
	}
	return nil
}

// DeleteNamespace retires the namespace; its records age out under the
// retention. Idempotent: a missing namespace is not an error.
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	_, err := c.Resources.Delete(ctx, connect.NewRequest(&managementv1.DeleteRequest{Ref: "namespace/" + name}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		return fmt.Errorf("graphene: delete namespace %s: %w", name, err)
	}
	return nil
}

// SetSecret writes a secret value into the namespace of ctx. The value is
// never readable back — the server keeps only the name.
func (c *Client) SetSecret(ctx context.Context, name, value string) error {
	_, err := c.Secrets.SetSecret(ctx, connect.NewRequest(&managementv1.SetSecretRequest{Name: name, Value: value}))
	if err != nil {
		return fmt.Errorf("graphene: set secret %s: %w", name, err)
	}
	return nil
}

// DeleteSecret removes a secret of the namespace of ctx.
func (c *Client) DeleteSecret(ctx context.Context, name string) error {
	_, err := c.Resources.Delete(ctx, connect.NewRequest(&managementv1.DeleteRequest{Ref: "secret/" + name}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		return fmt.Errorf("graphene: delete secret %s: %w", name, err)
	}
	return nil
}

// IsNotFound reports a Graphene not-found.
func IsNotFound(err error) bool { return connect.CodeOf(err) == connect.CodeNotFound }
