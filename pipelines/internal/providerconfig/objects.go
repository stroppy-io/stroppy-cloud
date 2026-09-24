package providerconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const crossplaneNamespace = "stroppy-provider-credentials"

type request struct {
	Provider spec.ProviderKind
	Name     string
	Settings json.RawMessage
}

func providerObjects(req request, credJSON string) (map[string][]byte, *unstructured.Unstructured, error) {
	switch req.Provider {
	case spec.ProviderYandex:
		var c struct {
			SAKeyJSON string `json:"sa_key_json"`
		}
		if err := json.Unmarshal([]byte(credJSON), &c); err != nil {
			return nil, nil, fmt.Errorf("yandex credentials: %w", err)
		}
		if strings.TrimSpace(c.SAKeyJSON) == "" {
			return nil, nil, fmt.Errorf("yandex credentials: sa_key_json is empty")
		}
		var st struct {
			FolderID string `json:"folder_id"`
			CloudID  string `json:"cloud_id"`
		}
		if len(req.Settings) > 0 {
			if err := json.Unmarshal(req.Settings, &st); err != nil {
				return nil, nil, fmt.Errorf("yandex settings: %w", err)
			}
		}
		creds := map[string]any{
			"source":    "Secret",
			"secretRef": map[string]any{"namespace": crossplaneNamespace, "name": req.Name, "key": "credentials"},
		}
		if st.FolderID != "" {
			creds["folderId"] = st.FolderID
		}
		if st.CloudID != "" {
			creds["cloudId"] = st.CloudID
		}
		pc := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "yandex-cloud.jet.crossplane.io/v1beta1",
			"kind":       "ProviderConfig",
			"metadata":   map[string]any{"name": req.Name},
			"spec":       map[string]any{"credentials": creds},
		}}
		return map[string][]byte{"credentials": []byte(c.SAKeyJSON)}, pc, nil
	case spec.ProviderAWS:
		var c struct {
			AccessKeyID     string `json:"access_key_id"`
			SecretAccessKey string `json:"secret_access_key"`
			SessionToken    string `json:"session_token"`
		}
		if err := json.Unmarshal([]byte(credJSON), &c); err != nil {
			return nil, nil, fmt.Errorf("aws credentials: %w", err)
		}
		if c.AccessKeyID == "" || c.SecretAccessKey == "" {
			return nil, nil, fmt.Errorf("aws credentials: access_key_id and secret_access_key are required")
		}
		ini := "[default]\naws_access_key_id = " + c.AccessKeyID + "\naws_secret_access_key = " + c.SecretAccessKey + "\n"
		if c.SessionToken != "" {
			ini += "aws_session_token = " + c.SessionToken + "\n"
		}
		pc := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "aws.upbound.io/v1beta1",
			"kind":       "ProviderConfig",
			"metadata":   map[string]any{"name": req.Name},
			"spec": map[string]any{"credentials": map[string]any{
				"source":    "Secret",
				"secretRef": map[string]any{"namespace": crossplaneNamespace, "name": req.Name, "key": "creds"},
			}},
		}}
		return map[string][]byte{"creds": []byte(ini)}, pc, nil
	default:
		return nil, nil, fmt.Errorf("ensure provider config: unsupported provider %q", req.Provider)
	}
}
