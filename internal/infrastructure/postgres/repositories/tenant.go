package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// TenantRepo stores tenants, members and invites.
type TenantRepo struct {
	q *db.Queries
}

var _ tenant.Repository = (*TenantRepo)(nil)

// NewTenantRepo builds the repo.
func NewTenantRepo(database tx.DB) *TenantRepo { return &TenantRepo{q: db.New(database)} }

func (r *TenantRepo) Insert(ctx context.Context, t tenant.Tenant) error {
	err := r.q.InsertTenant(ctx, db.InsertTenantParams{
		ID: t.ID, Slug: t.Slug, Name: t.Name, Description: t.Description, OwnerID: t.OwnerID, GrapheneNamespace: t.GrapheneNamespace,
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("slug is taken")
		}
		return infraf("tenant: insert: %v", err)
	}
	return nil
}

func (r *TenantRepo) BySlug(ctx context.Context, slug string) (tenant.Tenant, error) {
	row, err := r.q.TenantBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenant.Tenant{}, errs.NotFound("tenant")
		}
		return tenant.Tenant{}, infraf("tenant: by slug: %v", err)
	}
	return tenantRow(db.TenantByIDRow(row)), nil
}

func (r *TenantRepo) ByID(ctx context.Context, id uuid.UUID) (tenant.Tenant, error) {
	row, err := r.q.TenantByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenant.Tenant{}, errs.NotFound("tenant")
		}
		return tenant.Tenant{}, infraf("tenant: by id: %v", err)
	}
	return tenantRow(row), nil
}

func (r *TenantRepo) OfUser(ctx context.Context, userID uuid.UUID) ([]tenant.Membership, error) {
	rows, err := r.q.TenantsOfUser(ctx, userID)
	if err != nil {
		return nil, infraf("tenant: of user: %v", err)
	}
	out := make([]tenant.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenant.Membership{
			Tenant: tenantRow(db.TenantByIDRow{
				ID: row.ID, Slug: row.Slug, Name: row.Name, Description: row.Description, PublicName: row.PublicName,
				Status: row.Status, OwnerID: row.OwnerID, GrapheneNamespace: row.GrapheneNamespace,
				CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, MemberCount: row.MemberCount,
			}),
			Role: tenant.Role(row.Role), JoinedAt: row.JoinedAt,
		})
	}
	return out, nil
}

func (r *TenantRepo) OwnedBy(ctx context.Context, userID uuid.UUID) (id uuid.UUID, slug string, owned bool, err error) {
	row, err := r.q.OwnedTenant(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", false, nil
		}
		return uuid.Nil, "", false, infraf("tenant: owned by: %v", err)
	}
	return row.ID, row.Slug, true, nil
}

func (r *TenantRepo) SlugTaken(ctx context.Context, slug string) (bool, error) {
	if _, err := r.q.SlugTaken(ctx, slug); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, infraf("tenant: slug taken: %v", err)
	}
	return true, nil
}

func (r *TenantRepo) Update(ctx context.Context, id uuid.UUID, p tenant.Patch) error {
	n, err := r.q.UpdateTenant(ctx, db.UpdateTenantParams{
		ID: id, Name: p.Name, Description: p.Description, PublicName: p.PublicName, ClearPublicName: p.ClearPublicName,
	})
	if err != nil {
		return infraf("tenant: update: %v", err)
	}
	if n == 0 {
		return errs.NotFound("tenant")
	}
	return nil
}

func (r *TenantRepo) SetOwner(ctx context.Context, id, ownerID uuid.UUID) error {
	if _, err := r.q.SetTenantOwner(ctx, db.SetTenantOwnerParams{ID: id, OwnerID: ownerID}); err != nil {
		if isUnique(err) {
			return errs.Conflict("the new owner already owns a tenant")
		}
		return infraf("tenant: set owner: %v", err)
	}
	return nil
}

func (r *TenantRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.SoftDeleteTenant(ctx, id)
	if err != nil {
		return infraf("tenant: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("tenant")
	}
	return nil
}

// --- members --------------------------------------------------------------

func (r *TenantRepo) UpsertMember(ctx context.Context, tenantID, userID uuid.UUID, role tenant.Role) error {
	if err := r.q.UpsertMember(ctx, db.UpsertMemberParams{TenantID: tenantID, UserID: userID, Role: string(role)}); err != nil {
		return infraf("member: upsert: %v", err)
	}
	return nil
}

func (r *TenantRepo) MemberRole(ctx context.Context, tenantID, userID uuid.UUID) (tenant.Role, bool, error) {
	row, err := r.q.MemberRole(ctx, db.MemberRoleParams{TenantID: tenantID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, infraf("member: role: %v", err)
	}
	return tenant.Role(row.Role), true, nil
}

func (r *TenantRepo) Members(ctx context.Context, tenantID uuid.UUID) ([]tenant.Member, error) {
	rows, err := r.q.MembersOfTenant(ctx, tenantID)
	if err != nil {
		return nil, infraf("member: list: %v", err)
	}
	out := make([]tenant.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenant.Member{
			UserID: row.UserID, DisplayName: row.DisplayName, Avatar: row.Avatar, Email: row.Email,
			Role: tenant.Role(row.Role), JoinedAt: row.JoinedAt, LastSeenAt: row.LastSeenAt,
		})
	}
	return out, nil
}

func (r *TenantRepo) SetMemberRole(ctx context.Context, tenantID, userID uuid.UUID, role tenant.Role) error {
	n, err := r.q.SetMemberRole(ctx, db.SetMemberRoleParams{TenantID: tenantID, UserID: userID, Role: string(role)})
	if err != nil {
		return infraf("member: set role: %v", err)
	}
	if n == 0 {
		return errs.NotFound("member")
	}
	return nil
}

func (r *TenantRepo) TouchMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	return r.q.TouchMember(ctx, db.TouchMemberParams{TenantID: tenantID, UserID: userID})
}

func (r *TenantRepo) DeleteMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	n, err := r.q.DeleteMember(ctx, db.DeleteMemberParams{TenantID: tenantID, UserID: userID})
	if err != nil {
		return infraf("member: delete: %v", err)
	}
	if n == 0 {
		return errs.NotFound("member")
	}
	return nil
}

// --- invites --------------------------------------------------------------

func (r *TenantRepo) InsertInvite(ctx context.Context, inv tenant.Invite) error {
	err := r.q.InsertInvite(ctx, db.InsertInviteParams{
		ID: inv.ID, TenantID: inv.TenantID, Email: inv.Email, Role: string(inv.Role), InvitedBy: inv.InvitedBy,
		Message: inv.Message, ExpiresAt: inv.ExpiresAt,
	})
	if err != nil {
		if isUnique(err) {
			return errs.Conflict("a pending invite for this email exists")
		}
		return infraf("invite: insert: %v", err)
	}
	return nil
}

func (r *TenantRepo) InviteByID(ctx context.Context, id uuid.UUID) (tenant.Invite, error) {
	row, err := r.q.InviteByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenant.Invite{}, errs.NotFound("invite")
		}
		return tenant.Invite{}, infraf("invite: by id: %v", err)
	}
	return inviteRow(row), nil
}

func (r *TenantRepo) PendingInvites(ctx context.Context, tenantID uuid.UUID) ([]tenant.Invite, error) {
	rows, err := r.q.PendingInvitesOfTenant(ctx, tenantID)
	if err != nil {
		return nil, infraf("invite: of tenant: %v", err)
	}
	out := make([]tenant.Invite, 0, len(rows))
	for _, row := range rows {
		out = append(out, inviteRow(db.InviteByIDRow(row)))
	}
	return out, nil
}

func (r *TenantRepo) PendingInvitesForEmail(ctx context.Context, email string) ([]tenant.Invite, error) {
	rows, err := r.q.PendingInvitesForEmail(ctx, email)
	if err != nil {
		return nil, infraf("invite: for email: %v", err)
	}
	out := make([]tenant.Invite, 0, len(rows))
	for _, row := range rows {
		out = append(out, inviteRow(db.InviteByIDRow(row)))
	}
	return out, nil
}

func (r *TenantRepo) ResolveInvite(ctx context.Context, id uuid.UUID, status tenant.InviteStatus) (bool, error) {
	n, err := r.q.ResolveInvite(ctx, db.ResolveInviteParams{ID: id, Status: string(status)})
	if err != nil {
		return false, infraf("invite: resolve: %v", err)
	}
	return n > 0, nil
}

func tenantRow(row db.TenantByIDRow) tenant.Tenant {
	t := tenant.Tenant{
		ID: row.ID, Slug: row.Slug, Name: row.Name, Description: row.Description, PublicName: row.PublicName,
		Status: tenant.Status(row.Status), OwnerID: row.OwnerID, GrapheneNamespace: row.GrapheneNamespace,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, MemberCount: int(row.MemberCount),
	}
	return t
}

func inviteRow(row db.InviteByIDRow) tenant.Invite {
	return tenant.Invite{
		ID: row.ID, TenantID: row.TenantID, Email: row.Email, Role: tenant.Role(row.Role), Status: tenant.InviteStatus(row.Status),
		InvitedBy: row.InvitedBy, InviterName: row.InviterName, InviterAvatar: row.InviterAvatar,
		Message: row.Message, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
	}
}

// Namespaces lists every live tenant's Graphene namespace.
func (r *TenantRepo) Namespaces(ctx context.Context) ([]string, error) {
	rows, err := r.q.TenantNamespaces(ctx)
	if err != nil {
		return nil, infraf("tenant: namespaces: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.GrapheneNamespace)
	}
	return out, nil
}
