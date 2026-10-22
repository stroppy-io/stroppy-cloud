package providerconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const (
	privateSettings = `{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-d","public_ips":false,"network":{"kind":"create"}}`
	publicSettings  = `{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-d","public_ips":true,"network":{"kind":"create"}}`
)

// converging makes every created vpc object report what Crossplane reports
// once the cloud has it: Ready, and an id. refuse turns that into a quota
// refusal instead.
func converging(cli fakeAdapter, refuse string) {
	cli.client.PrependReactor("create", "*", func(a ktesting.Action) (bool, runtime.Object, error) {
		u, ok := a.(ktesting.CreateAction).GetObject().(*unstructured.Unstructured)
		if !ok || u.GetAPIVersion() != "vpc.yandex-cloud.jet.crossplane.io/v1alpha1" {
			return false, nil, nil
		}
		conditions := []any{map[string]any{"type": "Ready", "status": "True"}, map[string]any{"type": "Synced", "status": "True"}}
		if refuse != "" {
			conditions = []any{map[string]any{"type": "Synced", "status": "False", "reason": "ReconcileError", "message": refuse}}
		}
		u.Object["status"] = map[string]any{"conditions": conditions, "atProvider": map[string]any{"id": "enp-" + u.GetKind()}}
		return false, nil, nil
	})
}

func get(t *testing.T, cli fakeAdapter, kind, name string) (*unstructured.Unstructured, bool) {
	t.Helper()
	c, err := cli.ObjectClient(context.Background(), sharedObject(kind, name))
	require.NoError(t, err)
	u, err := c.Get(context.Background(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	require.NoError(t, err)
	return u, true
}

func TestProfileNetworkLifecycle(t *testing.T) {
	p, name, labels, cli := fixture(t)
	p.Settings = json.RawMessage(privateSettings)
	converging(cli, "")
	ctx := context.Background()
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
	require.NoError(t, ensureNetwork(ctx, cli, p, name, labels))

	net, ok := get(t, cli, "Network", spec.ProfileNetworkName(name))
	require.True(t, ok)
	require.Equal(t, labels, net.GetLabels())
	ref, _, _ := unstructured.NestedString(net.Object, "spec", "providerConfigRef", "name")
	require.Equal(t, name, ref)
	folder, _, _ := unstructured.NestedString(net.Object, "spec", "forProvider", "folderId")
	require.Equal(t, "b1gia87mbaomkfvsleds", folder)
	routes, ok := get(t, cli, "RouteTable", spec.ProfileRouteTableName(name))
	require.True(t, ok)
	route, _, _ := unstructured.NestedSlice(routes.Object, "spec", "forProvider", "staticRoute")
	require.Equal(t, "0.0.0.0/0", route[0].(map[string]any)["destinationPrefix"])
	require.Equal(t, spec.ProfileGatewayName(name), route[0].(map[string]any)["gatewayIdRef"].(map[string]any)["name"])
	_, ok = get(t, cli, "Gateway", spec.ProfileGatewayName(name))
	require.True(t, ok)

	// Idempotent; the profile's own network is not a use of its config.
	require.NoError(t, ensureNetwork(ctx, cli, p, name, labels))
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))

	// A run joins it: the check hands out the shared route table.
	id, err := sharedNetwork(ctx, cli, p, name, labels)
	require.NoError(t, err)
	require.Equal(t, "enp-RouteTable", id)

	// A run subnet still in the network blocks deleting it.
	subnet := sharedObject("Subnet", "stroppy-acme-8f1c3f2a-subnet")
	subnet.Object["spec"] = map[string]any{"providerConfigRef": map[string]any{"name": name}}
	c, err := cli.ObjectClient(ctx, subnet)
	require.NoError(t, err)
	_, err = c.Create(ctx, subnet, metav1.CreateOptions{})
	require.NoError(t, err)
	require.ErrorContains(t, removeNetwork(ctx, cli, p.Provider, name, labels), "still in use by Subnet")
	_, ok = get(t, cli, "Network", spec.ProfileNetworkName(name))
	require.True(t, ok, "nothing deleted while a run uses the network")
	require.NoError(t, c.Delete(ctx, subnet.GetName(), metav1.DeleteOptions{}))

	require.NoError(t, removeNetwork(ctx, cli, p.Provider, name, labels))
	for _, o := range sharedNames(name) {
		_, ok := get(t, cli, o.kind, o.name)
		require.False(t, ok, o.name)
	}
	require.NoError(t, removeNetwork(ctx, cli, p.Provider, name, labels))
	require.NoError(t, remove(ctx, cli, p.Provider, name, labels))
	_, err = sharedNetwork(ctx, cli, p, name, labels)
	require.ErrorContains(t, err, "does not exist")
}

func TestProfileNetworkFollowsSettings(t *testing.T) {
	p, name, labels, cli := fixture(t)
	converging(cli, "")
	ctx := context.Background()
	p.Settings = json.RawMessage(privateSettings)
	require.NoError(t, ensureNetwork(ctx, cli, p, name, labels))

	// Public addresses: the shared NAT goes, the network stays.
	p.Settings = json.RawMessage(publicSettings)
	require.NoError(t, ensureNetwork(ctx, cli, p, name, labels))
	_, ok := get(t, cli, "Network", spec.ProfileNetworkName(name))
	require.True(t, ok)
	for _, kind := range []string{"Gateway", "RouteTable"} {
		_, ok := get(t, cli, kind, sharedNames(name)[map[string]int{"Gateway": 1, "RouteTable": 2}[kind]].name)
		require.False(t, ok, kind)
	}
	id, err := sharedNetwork(ctx, cli, p, name, labels)
	require.NoError(t, err)
	require.Empty(t, id)

	// An existing network is the user's: the created one goes, nothing new.
	p.Settings = json.RawMessage(`{"folder_id":"b1gia87mbaomkfvsleds","network":{"kind":"existing","network_id":"enp2v5nl4h1mabcdefgh","pool_cidr":"10.0.0.0/16","route_table_id":"enpq4v6ba7uhabcdefgh"}}`)
	require.NoError(t, ensureNetwork(ctx, cli, p, name, labels))
	_, ok = get(t, cli, "Network", spec.ProfileNetworkName(name))
	require.False(t, ok)
	id, err = sharedNetwork(ctx, cli, p, name, labels)
	require.NoError(t, err)
	require.Equal(t, "enpq4v6ba7uhabcdefgh", id)
}

func TestProfileNetworkRefusalAndOwnership(t *testing.T) {
	p, name, labels, cli := fixture(t)
	p.Settings = json.RawMessage(publicSettings)
	converging(cli, "rpc error: code = ResourceExhausted desc = Quota limit vpc.networks.count exceeded")
	ctx := context.Background()
	require.ErrorContains(t, ensureNetwork(ctx, cli, p, name, labels), "vpc.networks.count")

	_, name, _, cli = fixture(t)
	converging(cli, "")
	foreign := map[string]string{ownerLabel: "other", namespaceLabel: "t-other"}
	require.NoError(t, ensureNetwork(ctx, cli, p, name, foreign))
	require.ErrorContains(t, ensureNetwork(ctx, cli, p, name, labels), "another profile")
	_, err := sharedNetwork(ctx, cli, p, name, labels)
	require.ErrorContains(t, err, "not available")
	require.ErrorContains(t, removeNetwork(ctx, cli, p.Provider, name, labels), "another profile")
}

func TestAWSHasNoProfileNetwork(t *testing.T) {
	p, name, labels, cli := fixture(t)
	p.Provider = spec.ProviderAWS
	objects, err := networkObjects(p, name)
	require.NoError(t, err)
	require.Empty(t, objects)
	require.NoError(t, removeNetwork(context.Background(), cli, p.Provider, name, labels))
}

// A public profile upgraded while old runs still use its config: it never
// had a NAT pair, so there is nothing to wait for.
func TestPublicProfileNetworkNextToLiveRuns(t *testing.T) {
	p, name, labels, cli := fixture(t)
	p.Settings = json.RawMessage(publicSettings)
	converging(cli, "")
	ctx := context.Background()
	require.NoError(t, ensure(ctx, cli, p, name, labels, testCredentials))
	vm := sharedObject("Instance", "stroppy-acme-8f1c3f2a-db-1")
	vm.Object["spec"] = map[string]any{"providerConfigRef": map[string]any{"name": name}}
	c, err := cli.ObjectClient(ctx, vm)
	require.NoError(t, err)
	_, err = c.Create(ctx, vm, metav1.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, ensureNetwork(ctx, cli, p, name, labels))
	_, ok := get(t, cli, "Network", spec.ProfileNetworkName(name))
	require.True(t, ok)
	_, ok = get(t, cli, "Gateway", spec.ProfileGatewayName(name))
	require.False(t, ok)
}
