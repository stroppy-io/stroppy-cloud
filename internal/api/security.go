package api

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// Verifier turns a bearer token into an actor. Implemented in the
// application root (IAM chain + API tokens); the API only asks "who".
type Verifier interface {
	Verify(ctx context.Context, token string) (auth.Actor, error)
}

// Security is the ogen SecurityHandler.
type Security struct {
	verifier Verifier
}

var _ oas.SecurityHandler = (*Security)(nil)

// NewSecurity builds the security handler.
func NewSecurity(v Verifier) *Security { return &Security{verifier: v} }

// HandleBearerAuth resolves the actor and stores it in ctx. A rejected
// token is "unauthenticated" regardless of why — the reason is logged by
// the verifier, never returned to the caller.
func (s *Security) HandleBearerAuth(ctx context.Context, _ oas.OperationName, t oas.BearerAuth) (context.Context, error) {
	actor, err := s.verifier.Verify(ctx, t.Token)
	if err != nil {
		return ctx, errs.Unauthenticated("token rejected").Because("token_rejected")
	}
	return auth.WithActor(ctx, actor), nil
}

// actor returns the caller or an unauthenticated error (public operations
// never call this).
func actor(ctx context.Context) (auth.Actor, error) {
	a := auth.ActorFrom(ctx)
	if a.IsAnonymous() {
		return auth.Actor{}, errs.Unauthenticated("authentication required").Because("authentication_required")
	}
	return a, nil
}
