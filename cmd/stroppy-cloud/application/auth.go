package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	iamsdk "github.com/gopherex/iam/pkg/sdk"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	apitoken "github.com/stroppy-io/stroppy-cloud/internal/domain/token"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
)

/*
THE ACTOR comes ONLY from a verified token: login and registration live in
IAM; the server has no password table and never will.

Two passes, dispatched by token FORM:
  - personal API token `stc_...` — ours, hashed in the DB (lands with the
    tokens area);
  - IAM access token — a person.

Hybrid verification does not see revocations, so the IAM webhook writes
revoked sessions into a denylist the verifier consults; the window is the
project's access_ttl.
*/

// IAMConfig is the `iam` section.
type IAMConfig struct {
	Mode string `default:"hybrid" mapstructure:"mode"        validate:"oneof=remote local hybrid"`
	// BaseURL, ProjectID and ClientID are required unless dev mode is on.
	BaseURL     string `mapstructure:"base_url"                     validate:"omitempty,url"`
	Credential  string `mapstructure:"credential"`
	ProjectID   string `mapstructure:"project_id"`
	Environment string `default:"live"   mapstructure:"environment" validate:"required"`
	Issuer      string `mapstructure:"issuer"`
	Audience    string `mapstructure:"audience"`
	JWKSURL     string `mapstructure:"jwks_url"`
	// ClientID is the SPA's app client (X-Client-Id); served to the SPA
	// through the public config.
	ClientID string `mapstructure:"client_id"`
	// WebhookSigningSecret verifies IAM lifecycle events; empty = the
	// endpoint is not mounted.
	WebhookSigningSecret string `mapstructure:"webhook_signing_secret"`
	WebhookPath          string `default:"/webhooks/iam" mapstructure:"webhook_path" validate:"required"`
	// AccessTTL is the project's access-token lifetime: how long a revoked
	// session stays on the denylist.
	AccessTTL       time.Duration `default:"10m" mapstructure:"access_ttl"`
	JWKSCacheTTLSec int           `default:"3600" mapstructure:"jwks_cache_ttl_sec"`
	WarmTimeout     time.Duration `default:"5s" mapstructure:"warm_timeout"`
}

func (c *IAMConfig) WebhookEnabled() bool { return c.WebhookSigningSecret != "" }

// complete reports the settings IAM verification needs.
func (c *IAMConfig) complete() error {
	if c.BaseURL == "" || c.ProjectID == "" || c.ClientID == "" {
		return errors.New("iam.base_url, iam.project_id and iam.client_id are required (or turn dev mode on: dev.users)")
	}
	return nil
}

// DevConfig is the `dev` section: a local installation without IAM.
type DevConfig struct {
	// Users are static bearer tokens, `token=email[=Display name]`. Any
	// entry turns dev mode on: IAM is not asked, a token is a user.
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
// ensured and named on every call; invites for the e-mail apply as in IAM
// mode, so a local stand can rehearse membership flows.
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
// is IAM's.
type verifierChain struct {
	tokens *apitoken.Service
	iam    *iamVerifier
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
	if v.iam == nil {
		return auth.Actor{}, errUnauthenticated
	}
	return v.iam.Verify(ctx, token)
}

// iamVerifier verifies IAM access tokens. The IAM subject IS the profile
// id: no numbering of our own, it would diverge from IAM on day one. The
// profile is ensured on every verified call (cheap upsert) so the rest of
// the server can assume it exists.
type iamVerifier struct {
	auth     iamsdk.Authenticator
	users    *iam.Users
	denylist *repositories.IAMRepo
	profiles *profile.Service
	tenants  *tenant.Service
	log      *xlog.Logger
}

// newIAMVerifier builds the verifier and probes IAM. A failed warm-up does
// NOT disable verification: hybrid caches JWKS as soon as IAM answers.
func newIAMVerifier(
	ctx context.Context,
	cfg *IAMConfig,
	denylist *repositories.IAMRepo,
	profiles *profile.Service,
	tenants *tenant.Service,
	log *xlog.Logger,
) (v *iamVerifier, warmErr, err error) {
	authenticator, err := iamsdk.NewAuthenticator(iamsdk.AuthenticatorConfig{
		Mode:            iamsdk.ValidationMode(cfg.Mode),
		BaseURL:         cfg.BaseURL,
		Credential:      cfg.Credential,
		ProjectID:       cfg.ProjectID,
		Environment:     cfg.Environment,
		Issuer:          cfg.Issuer,
		Audience:        cfg.Audience,
		JWKSURL:         cfg.JWKSURL,
		JWKSCacheTTLSec: cfg.JWKSCacheTTLSec,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("iam authenticator: %w", err)
	}
	warmCtx, cancel := context.WithTimeout(ctx, cfg.WarmTimeout)
	defer cancel()
	if err := iamsdk.Warm(warmCtx, authenticator); err != nil {
		warmErr = fmt.Errorf("iam warm: %w", err)
	}
	return &iamVerifier{
		auth: authenticator, users: iam.NewUsers(cfg.BaseURL, cfg.ClientID, cfg.Environment),
		denylist: denylist, profiles: profiles, tenants: tenants, log: log,
	}, warmErr, nil
}

func (v *iamVerifier) Verify(ctx context.Context, token string) (auth.Actor, error) {
	principal, err := v.auth.Authenticate(ctx, token)
	if err != nil {
		// A rejected token is normal client state; a misconfigured verifier
		// lands here too, so keep it at debug — first place to look.
		v.log.Ctx().Debug(ctx, "iam: token rejected", xlog.ErrorCause(err))
		return auth.Actor{}, errUnauthenticated
	}
	user, err := uuid.Parse(principal.UserID)
	if err != nil {
		v.log.Ctx().Error(ctx, "iam: subject is not a uuid", xlog.String("subject", principal.UserID))
		return auth.Actor{}, errUnauthenticated
	}
	if principal.SessionID != "" {
		denied, err := v.denylist.SessionDenied(ctx, principal.SessionID)
		if err != nil {
			return auth.Actor{}, err
		}
		if denied {
			return auth.Actor{}, errUnauthenticated
		}
	}
	email, _ := principal.Claims["email"].(string) //nolint:errcheck // absent claim = empty email
	p, created, err := v.profiles.Ensure(ctx, user, email)
	if err != nil {
		return auth.Actor{}, err
	}
	if created || p.Email == "" {
		p = v.firstSight(ctx, token, user, p)
	}
	return auth.Actor{UserID: user, SessionID: principal.SessionID, Email: p.Email}, nil
}

// firstSight fills the profile from IAM users/me (the token carries no
// email) and applies invites waiting for that email. Failures degrade:
// the actor is still valid, the next call tries again.
func (v *iamVerifier) firstSight(ctx context.Context, token string, user uuid.UUID, p profile.Profile) profile.Profile {
	me, err := v.users.Me(ctx, token)
	if err != nil {
		v.log.Ctx().Warn(ctx, "iam: users/me failed", xlog.ErrorCause(err))
		return p
	}
	seeded, err := v.profiles.Seed(ctx, user, me.Email, me.Profile.Name, me.Profile.AvatarURL)
	if err != nil {
		v.log.Ctx().Warn(ctx, "profile: seed failed", xlog.ErrorCause(err))
		return p
	}
	if err := v.tenants.ApplyPending(ctx, user, seeded.Email); err != nil {
		v.log.Ctx().Warn(ctx, "invites: apply pending failed", xlog.ErrorCause(err))
	}
	return seeded
}
