package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/graphene-ci/pipeline/pkg/wire"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/providerconfig"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// NameEnsureProviderConfig is a read-only prerequisite check. Configuration is
// created by stroppy-provider-config and never by an individual workload run.
const NameEnsureProviderConfig = "stroppy.provider.check-config"

type EnsureProviderConfigRequest struct {
	Provider          spec.ProviderKind `json:"provider"`
	Name              string            `json:"name"`
	CredentialsSecret string            `json:"credentials_secret"`
	Settings          json.RawMessage   `json:"settings,omitempty"`
}
type EnsureProviderConfigResult struct {
	SecretName string `json:"secret_name"`
	Applied    bool   `json:"applied"`
	// RouteTableID is the profile network's route out for machines without
	// a public address (yandex), empty when it has none.
	RouteTableID string `json:"route_table_id,omitempty"`
}

func EnsureProviderConfig(ctx context.Context, req EnsureProviderConfigRequest) (EnsureProviderConfigResult, error) {
	profile := strings.TrimPrefix(req.CredentialsSecret, "provider-")
	if len(profile) > 36 {
		profile = profile[:36]
	}
	if req.Name != spec.ProviderConfigName(os.Getenv(wire.EnvNamespace), profile) {
		return EnsureProviderConfigResult{}, fmt.Errorf("provider config does not match this namespace and profile; configure the provider before launching")
	}
	p := spec.ProviderConfig{Provider: req.Provider, ProfileID: profile, CredentialsSecret: req.CredentialsSecret, Settings: req.Settings}
	if _, err := providerconfig.Check(ctx, p); err != nil {
		return EnsureProviderConfigResult{SecretName: req.Name}, err
	}
	// The run joins the profile network: it must be there before anything
	// of the run is declared in it.
	routeID, err := providerconfig.SharedNetwork(ctx, p)
	return EnsureProviderConfigResult{SecretName: req.Name, RouteTableID: routeID}, err
}
