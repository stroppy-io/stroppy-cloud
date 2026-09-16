package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-faster/jx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// ListProviderProfiles — profiles of the tenant.
func (h *Handler) ListProviderProfiles(ctx context.Context, params oas.ListProviderProfilesParams) (*oas.ListProviderProfilesOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	ps, err := h.deps.Providers.List(ctx, a, t.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.ListProviderProfilesOK{Data: make([]oas.ProviderProfile, 0, len(ps))}
	for _, p := range ps {
		out.Data = append(out.Data, profileOf(p))
	}
	return out, nil
}

// CreateProviderProfile — add a profile; verification runs in the background.
func (h *Handler) CreateProviderProfile(ctx context.Context, req *oas.ProviderProfileCreate, params oas.CreateProviderProfileParams) (*oas.ProviderProfile, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Providers.Create(ctx, a, t.ID, provider.Create{
		Name: req.Name, Kind: provider.Kind(req.Kind), Settings: rawOf(req.Settings), Credentials: rawOf(req.Credentials),
	})
	if err != nil {
		return nil, err
	}
	out := profileOf(p)
	return &out, nil
}

// GetProviderProfile — one profile.
func (h *Handler) GetProviderProfile(ctx context.Context, params oas.GetProviderProfileParams) (*oas.ProviderProfile, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Providers.Get(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	out := profileOf(p)
	return &out, nil
}

// PatchProviderProfile — rename, settings, credentials.
func (h *Handler) PatchProviderProfile(ctx context.Context, req *oas.ProviderProfilePatch, params oas.PatchProviderProfileParams) (*oas.ProviderProfile, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	patch := provider.Patch{}
	if v, ok := req.Name.Get(); ok {
		patch.Name = &v
	}
	if v, ok := req.Settings.Get(); ok {
		patch.Settings = rawOf(v)
	}
	if v, ok := req.Credentials.Get(); ok {
		patch.Credentials = rawOf(v)
	}
	p, err := h.deps.Providers.Update(ctx, a, t.ID, params.ID, patch)
	if err != nil {
		return nil, err
	}
	out := profileOf(p)
	return &out, nil
}

// DeleteProviderProfile — remove.
func (h *Handler) DeleteProviderProfile(ctx context.Context, params oas.DeleteProviderProfileParams) error {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Providers.Delete(ctx, a, t.ID, params.ID)
}

// VerifyProviderProfile — re-run verification.
func (h *Handler) VerifyProviderProfile(ctx context.Context, params oas.VerifyProviderProfileParams) (*oas.ProviderProfile, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Providers.Verify(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	out := profileOf(p)
	return &out, nil
}

// GetProviderQuotas — cached quotas.
func (h *Handler) GetProviderQuotas(ctx context.Context, params oas.GetProviderQuotasParams) (*oas.QuotaReport, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Providers.Quotas(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return quotaReportOf(p), nil
}

// RefreshProviderQuotas — probe now.
func (h *Handler) RefreshProviderQuotas(ctx context.Context, params oas.RefreshProviderQuotasParams) (*oas.QuotaReport, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	p, err := h.deps.Providers.RefreshQuotas(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	return quotaReportOf(p), nil
}

func rawOf(v oas.SchemaValue) json.RawMessage {
	if v == nil {
		return json.RawMessage("{}")
	}
	b, _ := json.Marshal(v) //nolint:errcheck // map of raw JSON always marshals
	return b
}

func schemaValueOf(raw json.RawMessage) oas.SchemaValue {
	raw = browserSchemaJSON(raw)
	out := oas.SchemaValue{}
	if len(raw) == 0 {
		return out
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}
	for k, v := range m {
		out[k] = jx.Raw(v)
	}
	return out
}

func profileOf(p provider.Profile) oas.ProviderProfile {
	out := oas.ProviderProfile{
		ID: p.ID, Name: p.Name, Kind: oas.ProviderKind(p.Kind), Status: oas.ProviderProfileStatus(p.Status),
		Settings: oas.NewOptSchemaValue(schemaValueOf(p.Settings)), SecretNames: p.SecretNames, CreatedAt: p.CreatedAt,
	}
	if out.SecretNames == nil {
		out.SecretNames = []string{}
	}
	if p.StatusReason != "" {
		out.StatusReason = oas.NewOptString(p.StatusReason)
	}
	if p.VerifiedAt != nil {
		out.VerifiedAt = oas.NewOptNilDateTime(*p.VerifiedAt)
	}
	if p.QuotasObservedAt != nil {
		out.QuotasObservedAt = oas.NewOptNilDateTime(*p.QuotasObservedAt)
	}
	if p.CreatedBy != nil {
		out.CreatedBy = oas.NewOptUserRef(oas.UserRef{ID: p.CreatedBy.String()})
	}
	return out
}

func quotaReportOf(p provider.Profile) *oas.QuotaReport {
	out := &oas.QuotaReport{Quotas: make([]oas.QuotaReportQuotasItem, 0, len(p.Quotas)), Stale: true}
	if p.QuotasUnavailableReason != "" {
		out.UnavailableReason = oas.NewOptString(p.QuotasUnavailableReason)
	}
	if p.QuotasScope != "" {
		out.Scope = oas.NewOptString(p.QuotasScope)
	}
	if p.QuotasObservedAt != nil {
		out.ObservedAt = oas.NewNilDateTime(*p.QuotasObservedAt)
		out.Stale = time.Since(*p.QuotasObservedAt) > provider.QuotaFreshness
	} else {
		out.ObservedAt.SetToNull()
	}
	for _, q := range p.Quotas {
		item := oas.QuotaReportQuotasItem{Name: q.Name, Limit: q.Limit, Used: q.Used}
		if q.Unit != "" {
			item.Unit = oas.NewOptString(q.Unit)
		}
		if q.Zone != "" {
			item.Zone = oas.NewOptString(q.Zone)
		}
		out.Quotas = append(out.Quotas, item)
	}
	return out
}
