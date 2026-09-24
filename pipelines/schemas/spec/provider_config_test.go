package spec

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestProviderConfig(t *testing.T) {
	id := "b413b185-61ca-4d81-abf2-0b760aa59ed5"
	schematest.Run(t, ProviderConfig(), schematest.Cases{Valid: []map[string]any{
		{"action": "ensure", "provider": "yandex", "profile_id": id, "credentials_secret": "provider-" + id, "settings": map[string]any{"folder_id": "folder"}},
		{"action": "delete", "provider": "yandex", "profile_id": id, "credentials_secret": "provider-" + id},
	}, Invalid: []schematest.Invalid{{Value: map[string]any{"action": "wipe", "provider": "yandex", "profile_id": id, "credentials_secret": "provider-" + id}, Code: "CHOICE_NOT_ALLOWED", Path: "action"}}})
}
