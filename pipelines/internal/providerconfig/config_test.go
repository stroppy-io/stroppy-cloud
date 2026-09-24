package providerconfig

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type fakeAdapter struct{ client *fake.FakeDynamicClient }

func (f fakeAdapter) ObjectClient(_ context.Context, u *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	gvr, _ := meta.UnsafeGuessKindToResource(u.GroupVersionKind())
	return f.client.Resource(gvr).Namespace(u.GetNamespace()), nil
}

func fixture(t *testing.T) (spec.ProviderConfig, string, map[string]string, fakeAdapter) {
	t.Helper()
	p := spec.ProviderConfig{Action: "ensure", Provider: spec.ProviderYandex, ProfileID: "b413b185-61ca-4d81-abf2-0b760aa59ed5", CredentialsSecret: "provider-b413b185-61ca-4d81-abf2-0b760aa59ed5"}
	name, labels, err := identity(p, "t-example")
	require.NoError(t, err)
	lists := map[schema.GroupVersionResource]string{}
	for _, k := range managedKinds(p.Provider) {
		u := object(k.version, k.kind, "", "")
		gvr, _ := meta.UnsafeGuessKindToResource(u.GroupVersionKind())
		lists[gvr] = k.kind + "List"
	}
	return p, name, labels, fakeAdapter{fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), lists)}
}

const testCredentials = `{"sa_key_json":"{\"id\":\"canary-private-credential\"}"}`

func TestProfileLifecycleAndRetry(t *testing.T) {
	p, name, labels, cli := fixture(t)
	ctx := context.Background()
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
	require.NoError(t, remove(ctx, cli, p.Provider, name, labels))
	require.NoError(t, remove(ctx, cli, p.Provider, name, labels))
}

func TestDeleteWaitsForProviderFinalizer(t *testing.T) {
	p, name, labels, cli := fixture(t)
	ctx := context.Background()
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
	cli.client.PrependReactor("delete", "providerconfigs", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, nil })
	limited, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, remove(limited, cli, p.Provider, name, labels), context.DeadlineExceeded)
	c, err := cli.ObjectClient(ctx, object("v1", "Secret", crossplaneNamespace, name))
	require.NoError(t, err)
	_, err = c.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err, "credentials must remain while config deletion is pending")
}

func TestEnsurePreservesControllerFinalizers(t *testing.T) {
	p, name, labels, cli := fixture(t)
	ctx := context.Background()
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
	c, err := cli.ObjectClient(ctx, configObject(p.Provider, name))
	require.NoError(t, err)
	u, err := c.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	u.SetFinalizers([]string{"in-use.crossplane.io"})
	u.SetAnnotations(map[string]string{"controller.example/managed": "true"})
	_, err = c.Update(ctx, u, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
	u, err = c.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"in-use.crossplane.io"}, u.GetFinalizers())
	require.Equal(t, "true", u.GetAnnotations()["controller.example/managed"])
}

func TestUsageAndForeignOwnershipBlockMutations(t *testing.T) {
	for _, kind := range []string{"Instance", "ProviderConfigUsage"} {
		t.Run(kind, func(t *testing.T) {
			p, name, labels, cli := fixture(t)
			ctx := context.Background()
			require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
			version := "compute.yandex-cloud.jet.crossplane.io/v1alpha1"
			if kind == "ProviderConfigUsage" {
				version = configObject(p.Provider, name).GetAPIVersion()
			}
			used := object(version, kind, "", "still-deleting")
			ref := map[string]any{"name": name}
			if kind == "ProviderConfigUsage" {
				used.Object["providerConfigRef"] = ref
			} else {
				used.Object["spec"] = map[string]any{"providerConfigRef": ref}
			}
			c, e := cli.ObjectClient(ctx, used)
			require.NoError(t, e)
			_, e = c.Create(ctx, used, metav1.CreateOptions{})
			require.NoError(t, e)
			require.ErrorContains(t, remove(ctx, cli, p.Provider, name, labels), "still in use")
			require.ErrorContains(t, ensure(ctx, cli, p, name, labels, testCredentials), "still in use")
		})
	}
	p, name, labels, cli := fixture(t)
	ctx := context.Background()
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
	foreign := map[string]string{ownerLabel: "other", namespaceLabel: "t-other"}
	require.ErrorContains(t, remove(ctx, cli, p.Provider, name, foreign), "another profile")
	require.ErrorContains(t, ensure(ctx, cli, p, name, foreign, testCredentials), "ownership conflict")
}

func TestIdentityAndCredentialErrors(t *testing.T) {
	p, name, labels, cli := fixture(t)
	other, _, err := identity(p, "t-other")
	require.NoError(t, err)
	require.NotEqual(t, name, other)
	p.CredentialsSecret = "foreign-key"
	_, _, err = identity(p, "t-example")
	require.Error(t, err)
	err = ensure(context.Background(), cli, p, name, labels, `{"sa_key_json":{"canary-private-credential":true}}`)
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "canary-private-credential"))
}
