// Package activities holds the activity BODIES of the stroppy pipelines:
// code that runs on an execution site — the run's worker (cluster-side
// work: credentials, k8s) or an agent's machine container (host prep,
// containers, stroppy). Workflow code in internal/run only names and
// dispatches them; nothing here touches workflow state.
//
// Every body is registered under a stable wire name (the Name* constants)
// through Register, which the pipeline's recording walk calls.
package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/graphene-ci/pipeline/pkg/obs"
	"github.com/graphene-ci/pipeline/pkg/workerapi"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// NameEnsureProviderConfig is the wire name of EnsureProviderConfig.
const NameEnsureProviderConfig = "stroppy.provider.ensure-config"

// crossplaneNamespace is where provider credential secrets live.
const crossplaneNamespace = "crossplane-system"

// KubeconfigSecret names the Graphene secret with the installation's
// kubeconfig. Both provider configuration and managed resources use it.
const KubeconfigSecret = "kubeconfig"

// EnsureProviderConfigRequest asks for the tenant's crossplane
// ProviderConfig and its credential Secret to exist and be current.
type EnsureProviderConfigRequest struct {
	Provider spec.ProviderKind `json:"provider"`
	// Name is the ProviderConfig name (t-<tenant>) — also the Secret name.
	Name string `json:"name"`
	// CredentialsSecret names the Graphene secret holding the cloud
	// credentials (provider.<kind>.credentials value as JSON).
	CredentialsSecret string `json:"credentials_secret"`
	// Settings is the baked provider settings (folder/cloud ids for yandex).
	Settings json.RawMessage `json:"settings,omitempty"`
}

// EnsureProviderConfigResult reports what was applied.
type EnsureProviderConfigResult struct {
	SecretName string `json:"secret_name"`
	Applied    bool   `json:"applied"`
}

// EnsureProviderConfig runs on the RUN worker (inside the cluster): resolves
// the Graphene secret, writes it as a Kubernetes Secret in crossplane-system
// and applies the provider's ProviderConfig pointing at it. Idempotent —
// the objects are named by the tenant profile, so concurrent runs converge
// on the same content; a rotated credential is picked up on the next run.
//
// This is deliberately NOT a Graphene resource: the ProviderConfig is shared
// by every run of the tenant, and an owned record would be cascaded away by
// the first run to finish.
func EnsureProviderConfig(ctx context.Context, req EnsureProviderConfigRequest) (EnsureProviderConfigResult, error) {
	if req.Name == "" || req.CredentialsSecret == "" {
		return EnsureProviderConfigResult{}, fmt.Errorf("ensure provider config: name and credentials_secret are required")
	}
	raw, err := workerapi.GetSecret(ctx, req.CredentialsSecret)
	if err != nil {
		return EnsureProviderConfigResult{}, err
	}
	data, pc, err := providerObjects(req, raw)
	if err != nil {
		return EnsureProviderConfigResult{}, err
	}
	cfg, err := kubeConfig(ctx, workerapi.GetSecret)
	if err != nil {
		return EnsureProviderConfigResult{}, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return EnsureProviderConfigResult{}, err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Name, Namespace: crossplaneNamespace,
			Labels: map[string]string{"app.kubernetes.io/managed-by": "stroppy-cloud", "stroppy.io/provider": string(req.Provider)},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	if err := upsertSecret(ctx, clientset, secret); err != nil {
		return EnsureProviderConfigResult{}, fmt.Errorf("secret %s: %w", req.Name, err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return EnsureProviderConfigResult{}, err
	}
	if err := upsertUnstructured(ctx, dyn, pc); err != nil {
		return EnsureProviderConfigResult{}, fmt.Errorf("providerconfig %s: %w", req.Name, err)
	}
	obs.Info(ctx, "provider config ensured", obs.Str("provider", string(req.Provider)), obs.Str("name", req.Name))
	return EnsureProviderConfigResult{SecretName: req.Name, Applied: true}, nil
}

// providerObjects builds the Secret data and the ProviderConfig of one
// provider from the Graphene credential value.
func providerObjects(req EnsureProviderConfigRequest, credJSON string) (map[string][]byte, *unstructured.Unstructured, error) {
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

// kubeConfig uses the same cluster and identity as k8slib.NewClientFromSecret.
// Selecting the pod's identity here can configure a different cluster, or fail
// under the default service account before managed resources are declared.
func kubeConfig(ctx context.Context, resolve func(context.Context, string) (string, error)) (*rest.Config, error) {
	raw, err := resolve(ctx, KubeconfigSecret)
	if err != nil {
		return nil, fmt.Errorf("kubernetes config secret %q: %w", KubeconfigSecret, err)
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("kubernetes config secret %q: %w", KubeconfigSecret, err)
	}
	return cfg, nil
}

func upsertSecret(ctx context.Context, cs kubernetes.Interface, s *corev1.Secret) error {
	existing, err := cs.CoreV1().Secrets(s.Namespace).Get(ctx, s.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		_, err = cs.CoreV1().Secrets(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
		return err
	case err != nil:
		return err
	}
	existing.Data = s.Data
	existing.Labels = s.Labels
	_, err = cs.CoreV1().Secrets(s.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func upsertUnstructured(ctx context.Context, dyn dynamic.Interface, obj *unstructured.Unstructured) error {
	gv, err := schema.ParseGroupVersion(obj.GetAPIVersion())
	if err != nil {
		return err
	}
	gvr := gv.WithResource(strings.ToLower(obj.GetKind()) + "s")
	res := dyn.Resource(gvr)
	existing, err := res.Get(ctx, obj.GetName(), metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		_, err = res.Create(ctx, obj, metav1.CreateOptions{})
		return err
	case err != nil:
		return err
	}
	obj.SetResourceVersion(existing.GetResourceVersion())
	_, err = res.Update(ctx, obj, metav1.UpdateOptions{})
	return err
}
