// Package token is the API tokens: personal (a person's key into one
// tenant, never stronger than their role) and service (a tenant's own key
// for CI, role member|viewer). Wire form `stc_<prefix>_<secret>`; only the
// sha256 of the secret is stored.
package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// Kind of token.
type Kind string

// Kinds.
const (
	KindPersonal Kind = "personal"
	KindService  Kind = "service"
)

const (
	wirePrefix = "stc_"
	prefixLen  = 8
	secretLen  = 32
)

// Token is the stored record (no secret).
type Token struct {
	ID         uuid.UUID
	Kind       Kind
	Name       string
	Prefix     string
	TenantID   uuid.UUID
	TenantSlug string
	TenantName string
	Role       string
	OwnerID    *uuid.UUID
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

// Stored is Token plus the hash, for the verifier.
type Stored struct {
	Token
	SecretHash []byte
}

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, t Token, secretHash []byte) error
	ByPrefix(ctx context.Context, prefix string) (Stored, error)
	OfOwner(ctx context.Context, ownerID uuid.UUID) ([]Token, error)
	ServiceOfTenant(ctx context.Context, tenantID uuid.UUID) ([]Token, error)
	// Revoke with optional owner/tenant confinement; false = nothing revoked.
	Revoke(ctx context.Context, id uuid.UUID, ownerID, tenantID *uuid.UUID) (bool, error)
	RevokeOfMember(ctx context.Context, tenantID, ownerID uuid.UUID) error
	Touch(ctx context.Context, id uuid.UUID) error
}

// Access resolves a caller's role in a tenant (the tenant service).
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug string, role string, err error)
}

// Service is the token use cases.
type Service struct {
	repo   Repository
	access Access
	audit  *audit.Service
}

// NewService builds the service.
func NewService(repo Repository, access Access, auditSvc *audit.Service) *Service {
	return &Service{repo: repo, access: access, audit: auditSvc}
}

// Created is a freshly minted token with its one-time secret.
type Created struct {
	Token
	Secret string
}

// CreatePersonal mints a personal token confined to one tenant the caller
// belongs to. Role defaults to the caller's role and can only be weaker.
func (s *Service) CreatePersonal(ctx context.Context, actor auth.Actor, name string, tenantID uuid.UUID, role string, expires *time.Time) (Created, error) {
	if actor.IsAPIToken() {
		return Created{}, errs.Forbidden("tokens cannot mint tokens").Because("tokens_cannot_mint_tokens")
	}
	slug, own, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return Created{}, err
	}
	if role == "" {
		role = own
	}
	if rank(role) == 0 || rank(role) > rank(own) {
		return Created{}, errs.Forbidden("token role cannot exceed your role").Because("token_role_cannot_exceed_your_role")
	}
	owner := actor.UserID
	return s.mint(ctx, Token{Kind: KindPersonal, Name: name, TenantID: tenantID, TenantSlug: slug, Role: role, OwnerID: &owner, ExpiresAt: expires})
}

// CreateService mints a tenant service token (admin+, role member|viewer).
func (s *Service) CreateService(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, name, role string, expires *time.Time) (Created, error) {
	slug, own, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return Created{}, err
	}
	if rank(own) < rank("admin") {
		return Created{}, errs.Forbidden("requires role admin").Because("requires_role_admin")
	}
	if role != "member" && role != "viewer" {
		return Created{}, errs.Invalid("role must be member or viewer").Because("role_must_be_member_or_viewer")
	}
	return s.mint(ctx, Token{Kind: KindService, Name: name, TenantID: tenantID, TenantSlug: slug, Role: role, ExpiresAt: expires})
}

func (s *Service) mint(ctx context.Context, t Token) (Created, error) {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" || len(t.Name) > 64 {
		return Created{}, errs.Invalid("name must be 1..64 characters").Because("name_must_be_1_64_characters")
	}
	if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
		return Created{}, errs.Invalid("expires_at is in the past").Because("expires_at_is_in_the_past")
	}
	t.ID = uuid.New()
	t.Prefix = randomToken(prefixLen)
	secret := randomToken(secretLen)
	t.CreatedAt = time.Now().UTC()
	if err := s.repo.Insert(ctx, t, hash(secret)); err != nil {
		return Created{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &t.TenantID, Action: "token.create", Target: audit.Target{Kind: "token", ID: t.ID.String(), Name: t.Name}, Details: map[string]any{"kind": t.Kind, "role": t.Role}}) //nolint:errcheck // audit never blocks
	return Created{Token: t, Secret: wirePrefix + t.Prefix + "_" + secret}, nil
}

// Mine lists the caller's personal tokens.
func (s *Service) Mine(ctx context.Context, actor auth.Actor) ([]Token, error) {
	return s.repo.OfOwner(ctx, actor.UserID)
}

// RevokeMine revokes a personal token of the caller.
func (s *Service) RevokeMine(ctx context.Context, actor auth.Actor, id uuid.UUID) error {
	owner := actor.UserID
	ok, err := s.repo.Revoke(ctx, id, &owner, nil)
	if err != nil {
		return err
	}
	if !ok {
		return errs.NotFound("token")
	}
	return nil
}

// ServiceTokens lists a tenant's service tokens (admin+).
func (s *Service) ServiceTokens(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) ([]Token, error) {
	if _, own, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return nil, err
	} else if rank(own) < rank("admin") {
		return nil, errs.Forbidden("requires role admin").Because("requires_role_admin")
	}
	return s.repo.ServiceOfTenant(ctx, tenantID)
}

// RevokeService revokes a tenant's service token (admin+).
func (s *Service) RevokeService(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if _, own, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return err
	} else if rank(own) < rank("admin") {
		return errs.Forbidden("requires role admin").Because("requires_role_admin")
	}
	ok, err := s.repo.Revoke(ctx, id, nil, &tenantID)
	if err != nil {
		return err
	}
	if !ok {
		return errs.NotFound("token")
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "token.revoke", Target: audit.Target{Kind: "token", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return nil
}

// RevokeOfMember is the membership hook.
func (s *Service) RevokeOfMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	return s.repo.RevokeOfMember(ctx, tenantID, userID)
}

// IsWire reports the `stc_` form.
func IsWire(token string) bool { return strings.HasPrefix(token, wirePrefix) }

// Verify resolves a wire token into an actor. Any failure is "not
// recognized": the form is not a hint worth giving.
func (s *Service) Verify(ctx context.Context, wire string) (auth.Actor, error) {
	rest := strings.TrimPrefix(wire, wirePrefix)
	prefix, secret, ok := strings.Cut(rest, "_")
	if !ok || len(prefix) != prefixLen || secret == "" {
		return auth.Actor{}, errs.Unauthenticated("token not recognized").Because("token_not_recognized")
	}
	stored, err := s.repo.ByPrefix(ctx, prefix)
	if err != nil {
		return auth.Actor{}, errs.Unauthenticated("token not recognized").Because("token_not_recognized")
	}
	if subtle.ConstantTimeCompare(stored.SecretHash, hash(secret)) != 1 {
		return auth.Actor{}, errs.Unauthenticated("token not recognized").Because("token_not_recognized")
	}
	if stored.ExpiresAt != nil && stored.ExpiresAt.Before(time.Now()) {
		return auth.Actor{}, errs.Unauthenticated("token expired").Because("token_expired")
	}
	_ = s.repo.Touch(ctx, stored.ID) //nolint:errcheck // best-effort
	a := auth.Actor{TokenID: stored.ID, TokenTenant: stored.TenantID, TokenRole: stored.Role}
	if stored.OwnerID != nil {
		a.UserID = *stored.OwnerID
	}
	return a, nil
}

func hash(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

var enc = base32.StdEncoding.WithPadding(base32.NoPadding)

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return strings.ToLower(enc.EncodeToString(b))[:n]
}

func rank(role string) int {
	switch role {
	case "owner":
		return 4
	case "admin":
		return 3
	case "member":
		return 2
	case "viewer":
		return 1
	}
	return 0
}
