package spec

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// ProviderConfig is the installation-owned provider configuration lifecycle.
// Values contain references only; cloud credentials never enter workflow history.
type ProviderConfig struct {
	Action            string          `json:"action"`
	Provider          ProviderKind    `json:"provider"`
	ProfileID         string          `json:"profile_id"`
	Settings          json.RawMessage `json:"settings,omitempty"`
	CredentialsSecret string          `json:"credentials_secret"`
}

// ProviderConfigName is stable, DNS-safe and isolated across Graphene namespaces.
func ProviderConfigName(namespace, profileID string) string {
	digest := sha256.Sum256([]byte(namespace))
	return fmt.Sprintf("sc-%x-%s", digest[:6], profileID)
}
