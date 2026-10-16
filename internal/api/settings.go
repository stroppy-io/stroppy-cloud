package api

import (
	"context"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// GetTenantSettings — defaults.
func (h *Handler) GetTenantSettings(ctx context.Context, params oas.GetTenantSettingsParams) (*oas.TenantSettings, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	st, err := h.deps.Settings.Get(ctx, a, t.ID)
	if err != nil {
		return nil, err
	}
	return settingsOf(st), nil
}

// PatchTenantSettings — update defaults.
func (h *Handler) PatchTenantSettings(ctx context.Context, req *oas.TenantSettingsPatch, params oas.PatchTenantSettingsParams) (*oas.TenantSettings, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p := settings.Patch{NotificationEmails: req.NotificationEmails}
	if v, ok := req.RunRetentionDays.Get(); ok {
		p.RunRetentionDays = &v
	}
	if v, ok := req.DefaultRating.Get(); ok {
		if b, ok := v.Tenant.Get(); ok {
			p.RatingTenant = &b
		}
		if b, ok := v.Global.Get(); ok {
			p.RatingGlobal = &b
		}
	}
	if v, ok := req.DefaultKeep.Get(); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, errs.Invalid("default_keep is not a duration").Because("default_keep_is_not_a_duration")
		}
		p.DefaultKeep = &d
	}
	st, err := h.deps.Settings.Update(ctx, a, t.ID, p)
	if err != nil {
		return nil, err
	}
	return settingsOf(st), nil
}

func settingsOf(st settings.Settings) *oas.TenantSettings {
	emails := st.NotificationEmails
	if emails == nil {
		emails = []string{}
	}
	return &oas.TenantSettings{
		RunRetentionDays:   st.RunRetentionDays,
		DefaultRating:      oas.RatingFlags{Tenant: oas.NewOptBool(st.RatingTenant), Global: oas.NewOptBool(st.RatingGlobal)},
		DefaultKeep:        st.DefaultKeep.String(),
		NotificationEmails: emails,
	}
}
