package api

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// ListMyTenants — tenants the caller belongs to.
func (h *Handler) ListMyTenants(ctx context.Context) (*oas.ListMyTenantsOK, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	ms, err := h.deps.Tenants.Mine(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	out := &oas.ListMyTenantsOK{Data: make([]oas.TenantMembership, 0, len(ms))}
	for _, m := range ms {
		out.Data = append(out.Data, membershipOf(m))
	}
	return out, nil
}

// CreateTenant — the caller's own tenant.
func (h *Handler) CreateTenant(ctx context.Context, req *oas.TenantCreate) (*oas.Tenant, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	if h.publicConfig(ctx).TenantCreation == "admin_only" && (h.deps.Admin == nil || !h.deps.Admin.IsAdmin(ctx, a)) {
		return nil, errs.Forbidden("tenant creation is restricted to platform admins").Because("tenant_creation_admin_only")
	}
	t, err := h.deps.Tenants.Create(ctx, a, tenant.Create{Name: req.Name, Slug: req.Slug.Or(""), Description: req.Description.Or("")})
	if err != nil {
		return nil, err
	}
	return tenantOf(t), nil
}

// SuggestTenantName — unique name/slug from the account.
func (h *Handler) SuggestTenantName(ctx context.Context, params oas.SuggestTenantNameParams) (*oas.SuggestTenantNameOK, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Profiles.Get(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	base := p.DisplayName
	if v, ok := params.Name.Get(); ok && strings.TrimSpace(v) != "" {
		base = v
	}
	name, slug, err := h.deps.Tenants.SuggestName(ctx, base, p.Email)
	if err != nil {
		return nil, err
	}
	return &oas.SuggestTenantNameOK{Name: name, Slug: slug}, nil
}

// GetTenant — details.
func (h *Handler) GetTenant(ctx context.Context, params oas.GetTenantParams) (*oas.Tenant, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	t, err := h.deps.Tenants.Get(ctx, a, params.Slug)
	if err != nil {
		return nil, err
	}
	return tenantOf(t), nil
}

// PatchTenant — rename / describe.
func (h *Handler) PatchTenant(ctx context.Context, req *oas.TenantPatch, params oas.PatchTenantParams) (*oas.Tenant, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	p := tenant.Patch{}
	if v, ok := req.Name.Get(); ok {
		p.Name = &v
	}
	if v, ok := req.Description.Get(); ok {
		p.Description = &v
	}
	if req.PublicName.Set {
		if req.PublicName.Null {
			p.ClearPublicName = true
		} else {
			v := req.PublicName.Value
			p.PublicName = &v
		}
	}
	t, err := h.deps.Tenants.Update(ctx, a, params.Slug, p)
	if err != nil {
		return nil, err
	}
	return tenantOf(t), nil
}

// DeleteTenant — retire.
func (h *Handler) DeleteTenant(ctx context.Context, params oas.DeleteTenantParams) error {
	a, err := actor(ctx)
	if err != nil {
		return err
	}
	return h.deps.Tenants.Delete(ctx, a, params.Slug)
}

// TransferTenant — ownership to another member.
func (h *Handler) TransferTenant(ctx context.Context, req *oas.TransferTenantReq, params oas.TransferTenantParams) (*oas.Tenant, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	to, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, errs.Invalid("user_id is not a uuid").Because("user_id_is_not_a_uuid")
	}
	t, err := h.deps.Tenants.Transfer(ctx, a, params.Slug, to)
	if err != nil {
		return nil, err
	}
	return tenantOf(t), nil
}

// ListMembers — members and roles.
func (h *Handler) ListMembers(ctx context.Context, params oas.ListMembersParams) (*oas.ListMembersOK, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	ms, err := h.deps.Tenants.Members(ctx, a, params.Slug)
	if err != nil {
		return nil, err
	}
	out := &oas.ListMembersOK{Data: make([]oas.Member, 0, len(ms))}
	for _, m := range ms {
		out.Data = append(out.Data, memberOf(m))
	}
	return out, nil
}

// PatchMember — change a role.
func (h *Handler) PatchMember(ctx context.Context, req *oas.PatchMemberReq, params oas.PatchMemberParams) (*oas.Member, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(params.UserId)
	if err != nil {
		return nil, errs.Invalid("userId is not a uuid").Because("user_id_is_not_a_uuid")
	}
	m, err := h.deps.Tenants.SetRole(ctx, a, params.Slug, userID, tenant.Role(req.Role))
	if err != nil {
		return nil, err
	}
	out := memberOf(m)
	return &out, nil
}

// RemoveMember — remove or leave.
func (h *Handler) RemoveMember(ctx context.Context, params oas.RemoveMemberParams) error {
	a, err := actor(ctx)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(params.UserId)
	if err != nil {
		return errs.Invalid("userId is not a uuid").Because("user_id_is_not_a_uuid")
	}
	return h.deps.Tenants.Remove(ctx, a, params.Slug, userID)
}

func tenantOf(t tenant.Tenant) *oas.Tenant {
	out := &oas.Tenant{
		ID: t.ID, Slug: t.Slug, Name: t.Name, Status: oas.TenantStatus(t.Status),
		Owner:             oas.UserRef{ID: t.OwnerID.String()},
		MemberCount:       oas.NewOptInt(t.MemberCount),
		CreatedAt:         t.CreatedAt,
		GrapheneNamespace: oas.NewOptString(t.GrapheneNamespace),
	}
	if t.Description != "" {
		out.Description = oas.NewOptString(t.Description)
	}
	if t.PublicName != nil {
		out.PublicName = oas.NewOptNilString(*t.PublicName)
	}
	return out
}

func membershipOf(m tenant.Membership) oas.TenantMembership {
	return oas.TenantMembership{Tenant: *tenantOf(m.Tenant), Role: oas.TenantRole(m.Role), JoinedAt: oas.NewOptDateTime(m.JoinedAt)}
}

func memberOf(m tenant.Member) oas.Member {
	out := oas.Member{
		User: oas.MemberUser{ID: m.UserID.String()},
		Role: oas.TenantRole(m.Role), JoinedAt: m.JoinedAt,
	}
	if m.DisplayName != "" {
		out.User.DisplayName = oas.NewOptString(m.DisplayName)
	}
	if m.Avatar != "" {
		out.User.Avatar = oas.NewOptString(m.Avatar)
	}
	if m.Email != "" {
		out.User.Email = oas.NewOptString(m.Email)
	}
	if m.LastSeenAt != nil {
		out.LastSeenAt = oas.NewOptNilDateTime(*m.LastSeenAt)
	}
	return out
}
