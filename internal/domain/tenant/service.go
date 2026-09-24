package tenant

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// InviteTTL is how long an invite stays pending.
const InviteTTL = 7 * 24 * time.Hour

// Service is the tenant use cases. Every mutation checks the caller's role
// here — the API only forwards.
type Service struct {
	repo      Repository
	ns        Namespaces
	tokens    Tokens
	tx        Transactor
	audit     *audit.Service
	runs      LiveRuns
	mail      Mailer
	push      Pipelines
	providers interface {
		DeleteTenant(context.Context, auth.Actor, uuid.UUID) error
	}
}

func (s *Service) WithProviderCleanup(p interface {
	DeleteTenant(context.Context, auth.Actor, uuid.UUID) error
},
) *Service {
	s.providers = p
	return s
}

// WithPipelines registers the pipeline push on tenant creation.
func (s *Service) WithPipelines(p Pipelines) *Service {
	s.push = p
	return s
}

// NewService builds the service.
func NewService(repo Repository, ns Namespaces, tokens Tokens, tx Transactor, auditSvc *audit.Service, runs LiveRuns) *Service {
	return &Service{repo: repo, ns: ns, tokens: tokens, tx: tx, audit: auditSvc, runs: runs}
}

// WithMailer enables invite e-mails.
func (s *Service) WithMailer(m Mailer) *Service {
	s.mail = m
	return s
}

// Access resolves the caller's role in the tenant: membership for people,
// the confinement for API tokens (a personal token never exceeds its
// owner's current role). Non-members get "not found", not "forbidden" —
// the tenant's existence is not theirs to learn.
func (s *Service) Access(ctx context.Context, actor auth.Actor, slug string) (Tenant, Role, error) {
	t, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return Tenant{}, "", err
	}
	if t.Status == StatusSuspended {
		return Tenant{}, "", errs.Forbidden("tenant is suspended")
	}
	if actor.IsAPIToken() {
		if actor.TokenTenant != t.ID {
			return Tenant{}, "", errs.NotFound("tenant")
		}
		role := Role(actor.TokenRole)
		if actor.UserID != uuid.Nil {
			own, ok, err := s.repo.MemberRole(ctx, t.ID, actor.UserID)
			if err != nil {
				return Tenant{}, "", err
			}
			if !ok {
				return Tenant{}, "", errs.NotFound("tenant")
			}
			role = role.Min(own)
		}
		return t, role, nil
	}
	role, ok, err := s.repo.MemberRole(ctx, t.ID, actor.UserID)
	if err != nil {
		return Tenant{}, "", err
	}
	if !ok {
		return Tenant{}, "", errs.NotFound("tenant")
	}
	_ = s.repo.TouchMember(ctx, t.ID, actor.UserID) //nolint:errcheck // presence is best-effort
	return t, role, nil
}

// Require is Access plus a role floor.
func (s *Service) Require(ctx context.Context, actor auth.Actor, slug string, required Role) (Tenant, Role, error) {
	t, role, err := s.Access(ctx, actor, slug)
	if err != nil {
		return Tenant{}, "", err
	}
	if !role.AtLeast(required) {
		return Tenant{}, "", errs.Forbidden(fmt.Sprintf("requires role %s", required))
	}
	return t, role, nil
}

// Mine lists the caller's tenants.
func (s *Service) Mine(ctx context.Context, userID uuid.UUID) ([]Membership, error) {
	return s.repo.OfUser(ctx, userID)
}

// Get returns the tenant for a member.
func (s *Service) Get(ctx context.Context, actor auth.Actor, slug string) (Tenant, error) {
	t, _, err := s.Access(ctx, actor, slug)
	return t, err
}

// Create makes the caller's own tenant: record + membership as owner +
// Graphene namespace. One owned tenant per account.
func (s *Service) Create(ctx context.Context, actor auth.Actor, req Create) (Tenant, error) {
	if actor.IsAPIToken() {
		return Tenant{}, errs.Forbidden("tokens cannot create tenants")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 64 {
		return Tenant{}, errs.Invalid("name must be 1..64 characters")
	}
	slug := req.Slug
	if slug == "" {
		slug = Slugify(name)
	}
	if !ValidSlug(slug) {
		return Tenant{}, errs.Invalid("slug must be 3..40 of [a-z0-9-], starting with a letter")
	}
	if _, _, owned, err := s.repo.OwnedBy(ctx, actor.UserID); err != nil {
		return Tenant{}, err
	} else if owned {
		return Tenant{}, errs.Conflict("account already owns a tenant")
	}
	if taken, err := s.repo.SlugTaken(ctx, slug); err != nil {
		return Tenant{}, err
	} else if taken {
		return Tenant{}, errs.Conflict(fmt.Sprintf("slug %q is taken", slug))
	}
	t := Tenant{
		ID:                uuid.New(),
		Slug:              slug,
		Name:              name,
		Description:       strings.TrimSpace(req.Description),
		Status:            StatusActive,
		OwnerID:           actor.UserID,
		GrapheneNamespace: "t-" + slug,
		MemberCount:       1,
		CreatedAt:         time.Now().UTC(),
	}
	// The namespace first: a record without a namespace is a tenant that
	// cannot run anything, while an orphaned namespace only costs retention.
	if err := s.ns.EnsureNamespace(ctx, t.GrapheneNamespace, map[string]string{
		"stroppy.io/tenant": t.ID.String(), "stroppy.io/slug": slug,
	}); err != nil {
		return Tenant{}, errs.Wrap(errs.CodeUnavailable, "graphene namespace", err)
	}
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.Insert(ctx, t); err != nil {
			return err
		}
		if err := s.repo.UpsertMember(ctx, t.ID, actor.UserID, RoleOwner); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TenantID: &t.ID, Action: "tenant.create", Target: audit.Target{Kind: "tenant", ID: t.ID.String(), Name: t.Slug}})
	})
	if err != nil {
		return Tenant{}, err
	}
	if s.push != nil {
		// The pipelines land in the new namespace in the background; the
		// first launch waits for it through the push state (admin status).
		go s.push.Sync(context.WithoutCancel(ctx), t.GrapheneNamespace)
	}
	return s.repo.ByID(ctx, t.ID)
}

// Update renames/describes (admin+).
func (s *Service) Update(ctx context.Context, actor auth.Actor, slug string, p Patch) (Tenant, error) {
	t, _, err := s.Require(ctx, actor, slug, RoleAdmin)
	if err != nil {
		return Tenant{}, err
	}
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" || len(n) > 64 {
			return Tenant{}, errs.Invalid("name must be 1..64 characters")
		}
		p.Name = &n
	}
	if err := s.repo.Update(ctx, t.ID, p); err != nil {
		return Tenant{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &t.ID, Action: "tenant.update", Target: audit.Target{Kind: "tenant", ID: t.ID.String(), Name: t.Slug}}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, t.ID)
}

// Delete retires the tenant (owner, no live runs): namespace retired,
// record soft-deleted, memberships and tokens gone with it.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, slug string) error {
	t, _, err := s.Require(ctx, actor, slug, RoleOwner)
	if err != nil {
		return err
	}
	if s.runs != nil {
		live, err := s.runs.HasLiveRuns(ctx, t.ID)
		if err != nil {
			return err
		}
		if live {
			return errs.Conflict("tenant has live runs")
		}
	}
	if s.providers != nil {
		if err := s.providers.DeleteTenant(ctx, actor, t.ID); err != nil {
			return err
		}
	}
	if err := s.ns.DeleteNamespace(ctx, t.GrapheneNamespace); err != nil {
		return errs.Wrap(errs.CodeUnavailable, "graphene namespace", err)
	}
	if s.push != nil {
		s.push.Forget(ctx, t.GrapheneNamespace)
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.SoftDelete(ctx, t.ID); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TenantID: &t.ID, Action: "tenant.delete", Target: audit.Target{Kind: "tenant", ID: t.ID.String(), Name: t.Slug}})
	})
}

// Transfer hands ownership to another member (owner). The old owner
// becomes admin.
func (s *Service) Transfer(ctx context.Context, actor auth.Actor, slug string, to uuid.UUID) (Tenant, error) {
	t, _, err := s.Require(ctx, actor, slug, RoleOwner)
	if err != nil {
		return Tenant{}, err
	}
	if to == actor.UserID {
		return Tenant{}, errs.Invalid("already the owner")
	}
	if _, ok, err := s.repo.MemberRole(ctx, t.ID, to); err != nil {
		return Tenant{}, err
	} else if !ok {
		return Tenant{}, errs.NotFound("member")
	}
	if _, _, owned, err := s.repo.OwnedBy(ctx, to); err != nil {
		return Tenant{}, err
	} else if owned {
		return Tenant{}, errs.Conflict("the new owner already owns a tenant")
	}
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.SetOwner(ctx, t.ID, to); err != nil {
			return err
		}
		if err := s.repo.SetMemberRole(ctx, t.ID, to, RoleOwner); err != nil {
			return err
		}
		if err := s.repo.SetMemberRole(ctx, t.ID, actor.UserID, RoleAdmin); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TenantID: &t.ID, Action: "tenant.transfer", Target: audit.Target{Kind: "user", ID: to.String()}})
	})
	if err != nil {
		return Tenant{}, err
	}
	return s.repo.ByID(ctx, t.ID)
}

// Members lists members (any member).
func (s *Service) Members(ctx context.Context, actor auth.Actor, slug string) ([]Member, error) {
	t, _, err := s.Access(ctx, actor, slug)
	if err != nil {
		return nil, err
	}
	return s.repo.Members(ctx, t.ID)
}

// SetRole changes a member's role (admin+; owner role only via Transfer;
// an admin cannot touch the owner).
func (s *Service) SetRole(ctx context.Context, actor auth.Actor, slug string, userID uuid.UUID, role Role) (Member, error) {
	t, _, err := s.Require(ctx, actor, slug, RoleAdmin)
	if err != nil {
		return Member{}, err
	}
	if !role.Valid() || role == RoleOwner {
		return Member{}, errs.Invalid("role must be admin, member or viewer")
	}
	if userID == t.OwnerID {
		return Member{}, errs.Forbidden("the owner's role changes only by transfer")
	}
	if _, ok, err := s.repo.MemberRole(ctx, t.ID, userID); err != nil {
		return Member{}, err
	} else if !ok {
		return Member{}, errs.NotFound("member")
	}
	if err := s.repo.SetMemberRole(ctx, t.ID, userID, role); err != nil {
		return Member{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &t.ID, Action: "member.role", Target: audit.Target{Kind: "user", ID: userID.String()}, Details: map[string]any{"role": role}}) //nolint:errcheck // audit never blocks
	return s.member(ctx, t.ID, userID)
}

// Remove drops a member (admin+), or lets the caller leave (self). The
// owner cannot leave; personal tokens of the member die with it.
func (s *Service) Remove(ctx context.Context, actor auth.Actor, slug string, userID uuid.UUID) error {
	// A personal token acts as its owner: leaving through it is still leaving.
	self := userID == actor.UserID
	required := RoleAdmin
	if self {
		required = RoleViewer
	}
	t, _, err := s.Require(ctx, actor, slug, required)
	if err != nil {
		return err
	}
	if userID == t.OwnerID {
		return errs.Forbidden("the owner cannot be removed; transfer ownership first")
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.DeleteMember(ctx, t.ID, userID); err != nil {
			return err
		}
		if err := s.tokens.RevokeOfMember(ctx, t.ID, userID); err != nil {
			return err
		}
		action := "member.remove"
		if self {
			action = "member.leave"
		}
		return s.audit.Record(ctx, audit.Entry{TenantID: &t.ID, Action: action, Target: audit.Target{Kind: "user", ID: userID.String()}})
	})
}

func (s *Service) member(ctx context.Context, tenantID, userID uuid.UUID) (Member, error) {
	members, err := s.repo.Members(ctx, tenantID)
	if err != nil {
		return Member{}, err
	}
	for _, m := range members {
		if m.UserID == userID {
			return m, nil
		}
	}
	return Member{}, errs.NotFound("member")
}

// --- invites --------------------------------------------------------------

// Invite offers membership by email (admin+).
func (s *Service) Invite(ctx context.Context, actor auth.Actor, slug, email string, role Role, message string) (Invite, error) {
	t, _, err := s.Require(ctx, actor, slug, RoleAdmin)
	if err != nil {
		return Invite{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(email) > 254 {
		return Invite{}, errs.Invalid("email is not valid")
	}
	if !role.Valid() || role == RoleOwner {
		return Invite{}, errs.Invalid("role must be admin, member or viewer")
	}
	inv := Invite{
		ID: uuid.New(), TenantID: t.ID, Email: email, Role: role, Status: InvitePending,
		Message: strings.TrimSpace(message), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(InviteTTL),
	}
	if !actor.IsAPIToken() {
		id := actor.UserID
		inv.InvitedBy = &id
	}
	if err := s.repo.InsertInvite(ctx, inv); err != nil {
		return Invite{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &t.ID, Action: "invite.create", Target: audit.Target{Kind: "invite", ID: inv.ID.String(), Name: email}}) //nolint:errcheck // audit never blocks
	if s.mail != nil {
		_ = s.mail.SendInvite(ctx, email, t, inv) //nolint:errcheck // the invite exists regardless; the mailer logs
	}
	return s.repo.InviteByID(ctx, inv.ID)
}

// Invites lists pending invites of a tenant (admin+).
func (s *Service) Invites(ctx context.Context, actor auth.Actor, slug string) ([]Invite, error) {
	t, _, err := s.Require(ctx, actor, slug, RoleAdmin)
	if err != nil {
		return nil, err
	}
	return s.repo.PendingInvites(ctx, t.ID)
}

// RevokeInvite withdraws a pending invite (admin+).
func (s *Service) RevokeInvite(ctx context.Context, actor auth.Actor, slug string, id uuid.UUID) error {
	t, _, err := s.Require(ctx, actor, slug, RoleAdmin)
	if err != nil {
		return err
	}
	inv, err := s.repo.InviteByID(ctx, id)
	if err != nil || inv.TenantID != t.ID {
		return errs.NotFound("invite")
	}
	ok, err := s.repo.ResolveInvite(ctx, id, InviteRevoked)
	if err != nil {
		return err
	}
	if !ok {
		return errs.Conflict("invite is not pending")
	}
	return s.audit.Record(ctx, audit.Entry{TenantID: &t.ID, Action: "invite.revoke", Target: audit.Target{Kind: "invite", ID: id.String(), Name: inv.Email}})
}

// MyInvites lists pending invites addressed to the caller's email.
func (s *Service) MyInvites(ctx context.Context, email string) ([]Invite, error) {
	if email == "" {
		return nil, nil
	}
	return s.repo.PendingInvitesForEmail(ctx, email)
}

// Accept joins the tenant of an invite addressed to the caller.
func (s *Service) Accept(ctx context.Context, actor auth.Actor, id uuid.UUID) (Membership, error) {
	inv, err := s.claimable(ctx, actor, id)
	if err != nil {
		return Membership{}, err
	}
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		ok, err := s.repo.ResolveInvite(ctx, id, InviteAccepted)
		if err != nil {
			return err
		}
		if !ok {
			return errs.Conflict("invite is not pending")
		}
		if err := s.repo.UpsertMember(ctx, inv.TenantID, actor.UserID, inv.Role); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TenantID: &inv.TenantID, Action: "invite.accept", Target: audit.Target{Kind: "invite", ID: id.String(), Name: inv.Email}})
	})
	if err != nil {
		return Membership{}, err
	}
	t, err := s.repo.ByID(ctx, inv.TenantID)
	if err != nil {
		return Membership{}, err
	}
	return Membership{Tenant: t, Role: inv.Role, JoinedAt: time.Now().UTC()}, nil
}

// Decline refuses an invite addressed to the caller.
func (s *Service) Decline(ctx context.Context, actor auth.Actor, id uuid.UUID) error {
	if _, err := s.claimable(ctx, actor, id); err != nil {
		return err
	}
	ok, err := s.repo.ResolveInvite(ctx, id, InviteDeclined)
	if err != nil {
		return err
	}
	if !ok {
		return errs.Conflict("invite is not pending")
	}
	return nil
}

// ApplyPending joins every pending invite for the email — the first login
// of an invited address.
func (s *Service) ApplyPending(ctx context.Context, userID uuid.UUID, email string) error {
	if email == "" {
		return nil
	}
	invites, err := s.repo.PendingInvitesForEmail(ctx, email)
	if err != nil {
		return err
	}
	for _, inv := range invites {
		if _, err := s.Accept(ctx, auth.Actor{UserID: userID, Email: email}, inv.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) claimable(ctx context.Context, actor auth.Actor, id uuid.UUID) (Invite, error) {
	if actor.IsAPIToken() {
		return Invite{}, errs.Forbidden("tokens cannot accept invites")
	}
	inv, err := s.repo.InviteByID(ctx, id)
	if err != nil {
		return Invite{}, err
	}
	if !strings.EqualFold(inv.Email, actor.Email) {
		return Invite{}, errs.NotFound("invite")
	}
	if inv.Status != InvitePending || inv.ExpiresAt.Before(time.Now()) {
		return Invite{}, errs.Conflict("invite is not pending")
	}
	return inv, nil
}

// --- naming ---------------------------------------------------------------

// SuggestName derives a unique name/slug for the caller's tenant from the
// display name or the email's local part.
func (s *Service) SuggestName(ctx context.Context, displayName, email string) (name, slug string, err error) {
	name = strings.TrimSpace(displayName)
	if name == "" && email != "" {
		name, _, _ = strings.Cut(email, "@")
	}
	if name == "" {
		name = "workspace"
	}
	slug = Slugify(name)
	if len(slug) < 3 {
		slug += "-team"
	}
	candidate := slug
	for i := 2; i < 100; i++ {
		taken, err := s.repo.SlugTaken(ctx, candidate)
		if err != nil {
			return "", "", err
		}
		if !taken {
			return name, candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", slug, i)
	}
	return name, slug + "-" + uuid.NewString()[:8], nil
}

// Slugify makes a slug: lowercase [a-z0-9-], letter first, ≤40.
func Slugify(s string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	for out != "" && (out[0] < 'a' || out[0] > 'z') {
		out = out[1:]
	}
	if len(out) > 40 {
		out = strings.TrimRight(out[:40], "-")
	}
	return out
}

// ValidSlug is the slug rule.
func ValidSlug(s string) bool {
	if len(s) < 3 || len(s) > 40 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}
