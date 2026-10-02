package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	apitoken "github.com/stroppy-io/stroppy-cloud/internal/domain/token"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/kratos"
)

/*
THE ACTOR comes ONLY from a verified token: login and registration live in
Kratos (deployments/kratos/); the server has no password table and never
will. The server never calls Kratos per request — the SPA exchanges its
Kratos session for a short-lived JWT (template `stroppy`) and the verifier
checks it locally against the public key set.

Two passes, dispatched by token FORM:
  - personal API token `stc_...` — ours, hashed in the DB;
  - Kratos session JWT — a person.

Revocation window = the JWT's TTL (10m): the SPA refreshes from the Kratos
session, a logged-out session stops refreshing. An e-mail is trusted only
with email_verified=true — invites and the admin bootstrap match by e-mail.
*/

// AuthConfig is the `auth` section.
type AuthConfig struct {
	kratos.Config `mapstructure:",squash"`
	// KratosPublicURL is the origin the browser reaches Kratos at; served
	// to the SPA through the public config. The server itself does not
	// call this URL.
	KratosPublicURL string `default:"http://localhost:4433" mapstructure:"kratos_public_url" validate:"omitempty,url"`
}

// DevConfig is the `dev` section: a local installation without Kratos.
type DevConfig struct {
	// Users are static bearer tokens, `token=email[=Display name]`. Any
	// entry turns dev mode on: Kratos is not asked, a token is a user.
	// NEVER for a shared installation — the tokens are the passwords.
	Users []string `mapstructure:"users"`
}

// Enabled reports dev mode.
func (c *DevConfig) Enabled() bool { return len(c.Users) > 0 }

// devUser is one static identity.
type devUser struct {
	id          uuid.UUID
	email, name string
}

// devNamespace makes a dev user's id stable across restarts.
var devNamespace = uuid.MustParse("6f9d1c02-5d0b-4d8e-9d44-5e0b2a1f7c11")

// devVerifier recognizes the static tokens of dev mode. The profile is
// ensured and named on every call; invites for the e-mail apply as in
// Kratos mode, so a local stand can rehearse membership flows.
type devVerifier struct {
	users    map[string]devUser
	profiles *profile.Service
	tenants  *tenant.Service
	log      *xlog.Logger
}

func newDevVerifier(cfg *DevConfig, profiles *profile.Service, tenants *tenant.Service, log *xlog.Logger) (*devVerifier, error) {
	v := &devVerifier{users: map[string]devUser{}, profiles: profiles, tenants: tenants, log: log}
	for _, entry := range cfg.Users {
		token, rest, ok := strings.Cut(entry, "=")
		email, name, _ := strings.Cut(rest, "=")
		if !ok || token == "" || !strings.Contains(email, "@") {
			return nil, fmt.Errorf("dev.users: %q is not token=email[=name]", entry)
		}
		if name == "" {
			name, _, _ = strings.Cut(email, "@")
		}
		v.users[token] = devUser{id: uuid.NewSHA1(devNamespace, []byte(strings.ToLower(email))), email: email, name: name}
	}
	return v, nil
}

func (v *devVerifier) Verify(ctx context.Context, token string) (auth.Actor, bool, error) {
	u, ok := v.users[token]
	if !ok {
		return auth.Actor{}, false, nil
	}
	p, created, err := v.profiles.Ensure(ctx, u.id, u.email)
	if err != nil {
		return auth.Actor{}, true, err
	}
	if created || p.DisplayName == "" {
		if _, err := v.profiles.Seed(ctx, u.id, u.email, u.name, ""); err != nil {
			v.log.Ctx().Warn(ctx, "dev: seed profile failed", xlog.ErrorCause(err))
		}
		if err := v.tenants.ApplyPending(ctx, u.id, u.email); err != nil {
			v.log.Ctx().Warn(ctx, "dev: apply invites failed", xlog.ErrorCause(err))
		}
	}
	return auth.Actor{UserID: u.id, Email: u.email}, true, nil
}

// errUnauthenticated — the token is not recognized. Never leaves the
// process with a reason attached.
var errUnauthenticated = errors.New("token not recognized")

// verifierChain dispatches by token form: `stc_` is ours, anything else
// must be a Kratos session JWT.
type verifierChain struct {
	tokens *apitoken.Service
	kratos *kratosVerifier
	// dev, when set, recognizes the static tokens of dev mode first.
	dev *devVerifier
}

func (v verifierChain) Verify(ctx context.Context, token string) (auth.Actor, error) {
	if token == "" {
		return auth.Actor{}, errUnauthenticated
	}
	if v.dev != nil {
		if a, ok, err := v.dev.Verify(ctx, token); ok {
			return a, err
		}
	}
	if apitoken.IsWire(token) {
		return v.tokens.Verify(ctx, token)
	}
	if v.kratos == nil {
		return auth.Actor{}, errUnauthenticated
	}
	return v.kratos.Verify(ctx, token)
}

// kratosVerifier verifies session JWTs. The Kratos identity id IS the
// profile id: no numbering of our own, it would diverge on day one. The
// profile is ensured on every verified call (cheap upsert) so the rest of
// the server can assume it exists. Display name and (once verified) e-mail
// are seeded from the token's claims — there is no users/me call.
type kratosVerifier struct {
	jwt      *kratos.Verifier
	profiles *profile.Service
	tenants  *tenant.Service
	log      *xlog.Logger
}

func newKratosVerifier(cfg *AuthConfig, profiles *profile.Service, tenants *tenant.Service, log *xlog.Logger) (*kratosVerifier, error) {
	jwtv, err := kratos.New(cfg.Config)
	if err != nil {
		return nil, err
	}
	return &kratosVerifier{jwt: jwtv, profiles: profiles, tenants: tenants, log: log}, nil
}

func (v *kratosVerifier) Verify(ctx context.Context, token string) (auth.Actor, error) {
	p, err := v.jwt.Verify(token)
	if err != nil {
		// A rejected token is normal client state; a misconfigured verifier
		// lands here too, so keep it at debug — first place to look.
		v.log.Ctx().Debug(ctx, "kratos: token rejected", xlog.ErrorCause(err))
		return auth.Actor{}, errUnauthenticated
	}
	user, err := uuid.Parse(p.Subject)
	if err != nil {
		v.log.Ctx().Error(ctx, "kratos: subject is not a uuid", xlog.String("subject", p.Subject))
		return auth.Actor{}, errUnauthenticated
	}
	prof, created, err := v.profiles.Ensure(ctx, user, p.Email)
	if err != nil {
		return auth.Actor{}, err
	}
	if created || (prof.Email == "" && p.Email != "") || prof.DisplayName == "" {
		if _, err := v.profiles.Seed(ctx, user, p.Email, p.Name, ""); err != nil {
			v.log.Ctx().Warn(ctx, "kratos: seed profile failed", xlog.ErrorCause(err))
		}
	}
	// Invites apply once the e-mail is verified; Ensure has recorded it.
	if p.EmailVerified {
		if err := v.tenants.ApplyPending(ctx, user, p.Email); err != nil {
			v.log.Ctx().Warn(ctx, "invites: apply pending failed", xlog.ErrorCause(err))
		}
	}
	return auth.Actor{UserID: user, SessionID: p.SessionID, Email: p.Email}, nil
}
