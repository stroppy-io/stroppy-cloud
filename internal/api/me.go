package api

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// GetMe — current profile, tenants and preferences.
func (h *Handler) GetMe(ctx context.Context) (*oas.Me, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Profiles.Get(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	return h.me(ctx, a, p), nil
}

// me is the profile as the caller sees it: platform admin by the database
// flag or by the installation's config list.
func (h *Handler) me(ctx context.Context, a auth.Actor, p profile.Profile) *oas.Me {
	me := meOf(p)
	if h.deps.Admin != nil && h.deps.Admin.IsAdmin(ctx, a) {
		me.IsPlatformAdmin = true
	}
	// Memberships travel with the profile so the SPA picks a tenant in one call.
	if h.deps.Tenants != nil {
		if ms, err := h.deps.Tenants.Mine(ctx, a.UserID); err == nil {
			me.Tenants = make([]oas.TenantMembership, 0, len(ms))
			for _, m := range ms {
				me.Tenants = append(me.Tenants, membershipOf(m))
				if m.Role == "owner" && m.Tenant.OwnerID == a.UserID {
					id := m.Tenant.ID
					me.OwnedTenantID = oas.NewOptNilUUID(id)
				}
			}
		}
	}
	return me
}

// PatchMe — display name, avatar, preferences, notifications.
func (h *Handler) PatchMe(ctx context.Context, req *oas.MePatch) (*oas.Me, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	patch := profile.Patch{}
	if v, ok := req.DisplayName.Get(); ok {
		patch.DisplayName = &v
	}
	if v, ok := req.Avatar.Get(); ok {
		patch.Avatar = &v
	}
	if v, ok := req.Preferences.Get(); ok {
		prefs := profile.Preferences{Theme: string(v.Theme.Or("")), Timezone: v.Timezone.Or("")}
		if dt, ok := v.DefaultTenant.Get(); ok {
			prefs.DefaultTenant = &dt
		}
		patch.Preferences = &prefs
	}
	if v, ok := req.Notifications.Get(); ok {
		n := profile.Notifications{}
		if b, ok := v.RunFinished.Get(); ok {
			n.RunFinished = &b
		}
		if b, ok := v.RunFailed.Get(); ok {
			n.RunFailed = &b
		}
		if b, ok := v.SuiteFinished.Get(); ok {
			n.SuiteFinished = &b
		}
		patch.Notifications = &n
	}
	p, err := h.deps.Profiles.Update(ctx, a.UserID, patch)
	if err != nil {
		return nil, err
	}
	return h.me(ctx, a, p), nil
}

func meOf(p profile.Profile) *oas.Me {
	me := &oas.Me{
		ID:              p.ID.String(),
		DisplayName:     p.DisplayName,
		IsPlatformAdmin: p.IsPlatformAdmin,
		Tenants:         []oas.TenantMembership{}, // tenants land with their area
		CreatedAt:       oas.NewOptDateTime(p.CreatedAt),
	}
	// The e-mail is served once Kratos confirmed the address.
	if p.Email != "" {
		me.Email = oas.NewOptString(p.Email)
	}
	if p.Avatar != "" {
		me.Avatar = oas.NewOptString(p.Avatar)
	}
	if p.Preferences.Theme != "" {
		me.Preferences.Theme = oas.NewOptPreferencesTheme(oas.PreferencesTheme(p.Preferences.Theme))
	}
	if p.Preferences.Timezone != "" {
		me.Preferences.Timezone = oas.NewOptString(p.Preferences.Timezone)
	}
	if p.Preferences.DefaultTenant != nil {
		me.Preferences.DefaultTenant = oas.NewOptNilString(*p.Preferences.DefaultTenant)
	}
	if p.Notifications.RunFinished != nil {
		me.Notifications.RunFinished = oas.NewOptBool(*p.Notifications.RunFinished)
	}
	if p.Notifications.RunFailed != nil {
		me.Notifications.RunFailed = oas.NewOptBool(*p.Notifications.RunFailed)
	}
	if p.Notifications.SuiteFinished != nil {
		me.Notifications.SuiteFinished = oas.NewOptBool(*p.Notifications.SuiteFinished)
	}
	return me
}
