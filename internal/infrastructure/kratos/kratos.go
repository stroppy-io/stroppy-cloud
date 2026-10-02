// Package kratos verifies the session JWTs minted by our Ory Kratos
// installation (template `stroppy`, see deployments/kratos/).
//
// The server NEVER calls Kratos per request: the SPA exchanges its Kratos
// session for a short-lived JWT (`GET /sessions/whoami?tokenize_as=stroppy`,
// TTL 10m) and sends it as a Bearer. Verification is local — signature
// against the public half of the signing key set, then iss/aud/exp.
//
// Claims contract (deployments/kratos/claims.jsonnet):
//
//	sub            Kratos identity id — IS the profile id
//	sid            Kratos session id
//	email          traits.email
//	email_verified verifiable_addresses[traits.email].verified
//	name           traits.name
//
// email is trusted ONLY with email_verified=true: tenant invites and the
// admin bootstrap match by e-mail, an unverified address must not claim them.
package kratos

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// Config is the `auth` section: what a JWT must carry to become an actor.
type Config struct {
	// JWKSFile is the public key set shared with Kratos (compose mounts
	// deployments/kratos/jwks). Rotation: put the new kid in first; the
	// file is re-read when its mtime moves.
	JWKSFile string `mapstructure:"jwks_file"`
	// JWKSURL serves the same key set over HTTPS (alternative to the file).
	JWKSURL string `mapstructure:"jwks_url" validate:"omitempty,url"`
	// Issuer and Audience are fixed by claims.jsonnet; defaults match it.
	Issuer   string `default:"stroppy-cloud" mapstructure:"issuer"   validate:"required"`
	Audience string `default:"stroppy-cloud" mapstructure:"audience" validate:"required"`
}

// complete reports whether verification can be built.
func (c *Config) complete() error {
	if c.JWKSFile == "" && c.JWKSURL == "" {
		return fmt.Errorf("auth.jwks_file or auth.jwks_url is required (or turn dev mode on: dev.users)")
	}
	return nil
}

// Principal is a verified token's payload.
type Principal struct {
	Subject       string // Kratos identity id (UUID)
	SessionID     string
	Email         string
	EmailVerified bool
	Name          string
}

// Verifier checks tokens locally.
type Verifier struct {
	cfg Config

	mu       sync.Mutex
	set      jwk.Set
	mtime    time.Time
	lastTry  time.Time // rate limit for failed refetches (url mode)
	minRetry time.Duration
}

// New loads the key set and returns the verifier.
func New(cfg Config) (*Verifier, error) {
	if err := cfg.complete(); err != nil {
		return nil, err
	}
	v := &Verifier{cfg: cfg, minRetry: 15 * time.Second}
	if _, err := v.load(); err != nil {
		return nil, fmt.Errorf("kratos jwks: %w", err)
	}
	return v, nil
}

// load (re)reads the key set; the file mode follows mtime, the url mode
// re-reads on a failed verification (rate-limited by the caller).
func (v *Verifier) load() (jwk.Set, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cfg.JWKSFile != "" {
		st, err := os.Stat(v.cfg.JWKSFile)
		if err != nil {
			return nil, err
		}
		if v.set != nil && !st.ModTime().After(v.mtime) {
			return v.set, nil
		}
		set, err := jwk.ReadFile(v.cfg.JWKSFile)
		if err != nil {
			return nil, err
		}
		v.set, v.mtime = set, st.ModTime()
		return set, nil
	}
	set, err := jwk.Fetch(context.Background(), v.cfg.JWKSURL)
	if err != nil {
		return nil, err
	}
	v.set = set
	return set, nil
}

// Verify checks the signature and claims and returns the principal.
// A bad token is an error; callers map any error to "unauthenticated".
func (v *Verifier) Verify(token string) (Principal, error) {
	p, err := v.verify(v.snapshot(), token)
	if err != nil {
		// The key set may have rotated: re-read once and try again.
		if set, lerr := v.reload(); lerr == nil {
			if p, err = v.verify(set, token); err == nil {
				return p, nil
			}
		}
		return Principal{}, err
	}
	return p, nil
}

func (v *Verifier) verify(set jwk.Set, token string) (Principal, error) {
	tok, err := jwt.Parse([]byte(token), jwt.WithKeySet(set))
	if err != nil {
		return Principal{}, fmt.Errorf("kratos: signature: %w", err)
	}
	if err := jwt.Validate(tok,
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithAudience(v.cfg.Audience),
		jwt.WithRequiredClaim(jwt.SubjectKey), jwt.WithRequiredClaim(jwt.IssuerKey),
	); err != nil {
		return Principal{}, fmt.Errorf("kratos: claims: %w", err)
	}
	p := Principal{}
	if p.Subject, _ = tok.Subject(); p.Subject == "" {
		return Principal{}, fmt.Errorf("kratos: empty subject")
	}
	var email, name string
	var verified bool
	_ = tok.Get("email", &email)             //nolint:errcheck // absent claim = empty
	_ = tok.Get("name", &name)               //nolint:errcheck // absent claim = empty
	_ = tok.Get("sid", &p.SessionID)         //nolint:errcheck // absent claim = empty
	_ = tok.Get("email_verified", &verified) //nolint:errcheck // absent = false
	if verified {
		p.Email = email // untrusted until Kratos confirmed the address
	}
	p.EmailVerified, p.Name = verified, name
	return p, nil
}

// snapshot returns the current key set without touching the disk.
func (v *Verifier) snapshot() jwk.Set {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.set
}

// reload re-reads the key set; in url mode it is rate-limited so a flood of
// bad tokens cannot turn into a flood of fetches.
func (v *Verifier) reload() (jwk.Set, error) {
	if v.cfg.JWKSFile == "" {
		v.mu.Lock()
		due := time.Since(v.lastTry) >= v.minRetry
		if due {
			v.lastTry = time.Now()
		}
		v.mu.Unlock()
		if !due {
			return nil, fmt.Errorf("kratos: refetch rate-limited")
		}
	}
	return v.load()
}
