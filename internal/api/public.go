package api

import (
	"context"

	"github.com/gopherex/xprobe"

	"github.com/stroppy-io/stroppy-cloud/internal/build"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// PublicConfig is what the SPA needs before login.
type PublicConfig struct {
	// AuthMode is "kratos" (the normal mode) or "dev".
	AuthMode string
	// KratosPublicURL is the origin the browser reaches Kratos at.
	KratosPublicURL string
	// TenantCreation is "anyone" or "admin_only".
	TenantCreation string
	PublicRating   bool
	Examples       bool
}

// GetPublicConfig — bootstrap config.
func (h *Handler) GetPublicConfig(ctx context.Context) (*oas.PublicConfig, error) {
	c := h.publicConfig(ctx)
	return &oas.PublicConfig{
		Auth:                oas.PublicConfigAuth{Mode: oas.PublicConfigAuthMode(c.AuthMode), Kratos: oas.PublicConfigAuthKratos{PublicURL: c.KratosPublicURL}},
		TenantCreation:      oas.PublicConfigTenantCreation(c.TenantCreation),
		PublicRatingEnabled: c.PublicRating,
		ExamplesEnabled:     oas.NewOptBool(c.Examples),
		Version:             build.Version,
		Commit:              oas.NewOptString(build.Commit),
	}, nil
}

// publicConfig is the static config overlaid with the system settings.
func (h *Handler) publicConfig(ctx context.Context) PublicConfig {
	c := h.deps.Public
	if h.deps.Admin == nil {
		return c
	}
	sys, err := h.deps.Admin.System(ctx)
	if err != nil {
		return c
	}
	c.TenantCreation, c.PublicRating, c.Examples = sys.TenantCreation, sys.PublicRatingEnabled, sys.ExamplesEnabled
	return c
}

// GetPublicHealth — server and dependencies.
func (h *Handler) GetPublicHealth(ctx context.Context) (*oas.Health, error) {
	out := &oas.Health{Status: oas.HealthStatusOk, Version: oas.NewOptString(build.Version), Components: oas.HealthComponents{}}
	for name, p := range h.deps.Probes {
		st := p.Check(ctx)
		item := oas.HealthComponentsItem{Status: oas.HealthComponentsItemStatusOk}
		if !st.OK() {
			item.Status = oas.HealthComponentsItemStatusDown
			item.Detail = oas.NewOptString(st.String())
			out.Status = oas.HealthStatusDegraded
		}
		out.Components[name] = item
	}
	return out, nil
}

// Probes are the named dependency probes for /public/health.
type Probes map[string]xprobe.Probe
