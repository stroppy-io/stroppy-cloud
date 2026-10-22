package providerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/wire"
	vpc "github.com/yandex-cloud/crossplane-provider-yc/apis/cluster/vpc/v1alpha1"
	"go.temporal.io/sdk/activity"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The profile network of a Yandex profile: one VPC network per profile
// (vpc.networks.count is the scarce quota), plus, when machines have no
// public addresses, one shared NAT gateway and a route table sending
// 0.0.0.0/0 through it. They live as long as the profile, outside every
// run's ownership tree: runs reference them by Crossplane name and add
// only their security group and zonal subnets. An existing network is the
// user's: nothing is created for it.
//
// doc: https://yandex.cloud/en/docs/vpc/concepts/network
// doc: https://yandex.cloud/en/docs/vpc/concepts/gateways
// doc: https://yandex.cloud/en/docs/vpc/concepts/routing

// networkReadyTimeout bounds waiting for one profile network object.
const networkReadyTimeout = 8 * time.Minute

// sharedNames are the Crossplane objects a profile may own besides its
// ProviderConfig, in creation order (deletion runs backwards).
func sharedNames(providerConfig string) []struct{ kind, name string } {
	return []struct{ kind, name string }{
		{vpc.Network_Kind, spec.ProfileNetworkName(providerConfig)},
		{vpc.Gateway_Kind, spec.ProfileGatewayName(providerConfig)},
		{vpc.RouteTable_Kind, spec.ProfileRouteTableName(providerConfig)},
	}
}

// ownNetwork marks the profile's own network objects, which do not count
// as usage of its ProviderConfig.
func ownNetwork(kind spec.ProviderKind, providerConfig string) map[string]bool {
	out := map[string]bool{}
	if kind != spec.ProviderYandex {
		return out
	}
	for _, o := range sharedNames(providerConfig) {
		out[o.kind+"/"+o.name] = true
	}
	return out
}

// networkObjects is what a profile's settings ask for, in creation order:
// nothing for AWS or an existing network.
func networkObjects(p spec.ProviderConfig, providerConfig string) ([]*unstructured.Unstructured, error) {
	if p.Provider != spec.ProviderYandex {
		return nil, nil
	}
	network, err := spec.YandexNetworkOf(p.Settings)
	if err != nil {
		return nil, err
	}
	if !network.Created() {
		return nil, nil
	}
	var st struct {
		FolderID string `json:"folder_id"`
	}
	if len(p.Settings) > 0 {
		if err := json.Unmarshal(p.Settings, &st); err != nil {
			return nil, fmt.Errorf("yandex settings: %w", err)
		}
	}
	if st.FolderID == "" {
		return nil, fmt.Errorf("yandex settings: folder_id is required for the profile network")
	}
	ref := xpv1.ResourceSpec{ProviderConfigReference: &xpv1.Reference{Name: providerConfig}}
	cloudLabels := map[string]*string{"stroppy-profile": ptr(p.ProfileID)}
	netName := spec.ProfileNetworkName(providerConfig)
	objects := []runtime.Object{&vpc.Network{
		TypeMeta:   metav1.TypeMeta{APIVersion: vpc.CRDGroupVersion.String(), Kind: vpc.Network_Kind},
		ObjectMeta: metav1.ObjectMeta{Name: netName},
		Spec: vpc.NetworkSpec{ResourceSpec: ref, ForProvider: vpc.NetworkParameters{
			FolderID: ptr(st.FolderID), Name: ptr(netName), Labels: cloudLabels,
			Description: ptr("stroppy-cloud provider profile network"),
		}},
	}}
	if network.SharedEgress() {
		natName, routesName := spec.ProfileGatewayName(providerConfig), spec.ProfileRouteTableName(providerConfig)
		objects = append(objects,
			&vpc.Gateway{
				TypeMeta:   metav1.TypeMeta{APIVersion: vpc.CRDGroupVersion.String(), Kind: vpc.Gateway_Kind},
				ObjectMeta: metav1.ObjectMeta{Name: natName},
				Spec: vpc.GatewaySpec{ResourceSpec: ref, ForProvider: vpc.GatewayParameters{
					FolderID: ptr(st.FolderID), Name: ptr(natName), Labels: cloudLabels,
					SharedEgressGateway: []vpc.SharedEgressGatewayParameters{{}},
				}},
			},
			&vpc.RouteTable{
				TypeMeta:   metav1.TypeMeta{APIVersion: vpc.CRDGroupVersion.String(), Kind: vpc.RouteTable_Kind},
				ObjectMeta: metav1.ObjectMeta{Name: routesName},
				Spec: vpc.RouteTableSpec{ResourceSpec: ref, ForProvider: vpc.RouteTableParameters{
					FolderID: ptr(st.FolderID), Name: ptr(routesName), Labels: cloudLabels,
					NetworkIDRef: &xpv1.Reference{Name: netName},
					StaticRoute: []vpc.StaticRouteParameters{{
						DestinationPrefix: ptr("0.0.0.0/0"), GatewayIDRef: &xpv1.Reference{Name: natName},
					}},
				}},
			})
	}
	out := make([]*unstructured.Unstructured, 0, len(objects))
	for _, o := range objects {
		raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
		if err != nil {
			return nil, err
		}
		out = append(out, &unstructured.Unstructured{Object: raw})
	}
	return out, nil
}

// sharedObject is an empty object of one shared kind, for lookups.
func sharedObject(kind, name string) *unstructured.Unstructured {
	return object(vpc.CRDGroupVersion.String(), kind, "", name)
}

// ConfigureNetwork is the profile network half of the lifecycle; it runs
// only on the Graphene worker.
func ConfigureNetwork(ctx context.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
	name, labels, err := identity(p, os.Getenv(wire.EnvNamespace))
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	cli := k8slib.NewClientInCluster()
	switch p.Action {
	case "ensure":
		err = ensureNetwork(ctx, cli, p, name, labels)
	case "delete":
		err = removeNetwork(ctx, cli, p.Provider, name, labels)
	default:
		err = fmt.Errorf("action must be ensure or delete")
	}
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	return spec.ProviderVerifyResult{OK: true}, nil
}

// ensureNetwork creates the objects the settings ask for and waits until
// each is Ready; objects the settings no longer ask for (the profile moved
// to an existing network or to public addresses) are deleted once no run
// uses the profile. An existing owned object is left as it is: its cloud
// side is what runs already joined.
func ensureNetwork(ctx context.Context, cli adapter, p spec.ProviderConfig, name string, labels map[string]string) error {
	want, err := networkObjects(p, name)
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	for _, u := range want {
		wanted[u.GetKind()+"/"+u.GetName()] = true
		if err := createOwned(ctx, cli, u, labels); err != nil {
			return err
		}
	}
	for _, u := range want {
		if err := waitReady(ctx, cli, u, labels); err != nil {
			return err
		}
	}
	if p.Provider != spec.ProviderYandex {
		return nil
	}
	shared := sharedNames(name)
	for i := len(shared) - 1; i >= 0; i-- {
		o := shared[i]
		if wanted[o.kind+"/"+o.name] {
			continue
		}
		if err := unused(ctx, cli, p.Provider, name, ownNetwork(p.Provider, name)); err != nil {
			return err
		}
		if err := deleteOwned(ctx, cli, sharedObject(o.kind, o.name), labels); err != nil {
			return err
		}
	}
	return nil
}

// removeNetwork deletes the profile network once no run object references
// the profile any more: route table, gateway, network, each waited out
// (the cloud refuses a network that still has subnets).
func removeNetwork(ctx context.Context, cli adapter, kind spec.ProviderKind, name string, labels map[string]string) error {
	if kind != spec.ProviderYandex {
		return nil
	}
	if err := unused(ctx, cli, kind, name, ownNetwork(kind, name)); err != nil {
		return err
	}
	shared := sharedNames(name)
	for i := len(shared) - 1; i >= 0; i-- {
		if err := deleteOwned(ctx, cli, sharedObject(shared[i].kind, shared[i].name), labels); err != nil {
			return err
		}
	}
	return nil
}

// createOwned creates u when absent; a present one must be the profile's.
func createOwned(ctx context.Context, cli adapter, u *unstructured.Unstructured, labels map[string]string) error {
	c, err := cli.ObjectClient(ctx, u)
	if err != nil {
		return err
	}
	old, err := c.Get(ctx, u.GetName(), metav1.GetOptions{})
	switch {
	case err == nil:
		if !owned(old, labels) {
			return fmt.Errorf("refusing to adopt %s %s owned by another profile", u.GetKind(), u.GetName())
		}
		if old.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%s %s is being deleted", u.GetKind(), u.GetName())
		}
		return nil
	case !apierrors.IsNotFound(err):
		return err
	}
	u.SetLabels(labels)
	_, err = c.Create(ctx, u, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return createOwned(ctx, cli, u, labels)
	}
	return err
}

// refusal is what the cloud says when it will not create the object on
// retry: an exhausted quota or missing rights (as in provision).
var refusal = regexp.MustCompile(`ResourceExhausted|Quota limit|[Qq]uota exceeded|PermissionDenied|[Pp]ermission denied`)

// waitReady polls u until its Ready condition is True (a route table also
// reports its cloud id), failing at once on a permanent cloud refusal.
func waitReady(ctx context.Context, cli adapter, u *unstructured.Unstructured, labels map[string]string) error {
	c, err := cli.ObjectClient(ctx, u)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, networkReadyTimeout)
	defer cancel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		live, err := c.Get(ctx, u.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if !owned(live, labels) {
			return fmt.Errorf("%s %s is owned by another profile", u.GetKind(), u.GetName())
		}
		ready, failure := readiness(live)
		if failure != "" {
			return fmt.Errorf("%s %s: %s", u.GetKind(), u.GetName(), failure)
		}
		if ready {
			return nil
		}
		if activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, "waiting for "+u.GetKind()+" "+u.GetName())
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s %s is not ready: %w", u.GetKind(), u.GetName(), ctx.Err())
		case <-ticker.C:
		}
	}
}

// readiness reads a managed object's conditions: ready, or the message of
// a permanent refusal.
func readiness(live *unstructured.Unstructured) (ready bool, failure string) {
	conditions, _, _ := unstructured.NestedSlice(live.Object, "status", "conditions") //nolint:errcheck // absent = not ready
	for _, raw := range conditions {
		c, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch c["type"] {
		case string(xpv1.TypeReady):
			ready = c["status"] == "True"
		case string(xpv1.TypeSynced):
			if msg, ok := c["message"].(string); ok && c["status"] == "False" && refusal.MatchString(msg) {
				failure = msg
			}
		}
	}
	if ready && live.GetKind() == vpc.RouteTable_Kind {
		id, _, _ := unstructured.NestedString(live.Object, "status", "atProvider", "id") //nolint:errcheck // absent = not ready
		ready = id != ""
	}
	return ready, failure
}

// SharedNetwork checks, for a run, that the profile network it joins is in
// place and returns the route table id machines without public addresses
// use (empty when the profile network has none).
func SharedNetwork(ctx context.Context, p spec.ProviderConfig) (string, error) {
	if p.Provider != spec.ProviderYandex {
		return "", nil
	}
	name, labels, err := identity(p, os.Getenv(wire.EnvNamespace))
	if err != nil {
		return "", err
	}
	return sharedNetwork(ctx, k8slib.NewClientInCluster(), p, name, labels)
}

func sharedNetwork(ctx context.Context, cli adapter, p spec.ProviderConfig, name string, labels map[string]string) (string, error) {
	network, err := spec.YandexNetworkOf(p.Settings)
	if err != nil {
		return "", err
	}
	if !network.Created() {
		return network.RouteTableID, nil
	}
	want, err := networkObjects(p, name)
	if err != nil {
		return "", err
	}
	routeID := ""
	for _, u := range want {
		c, err := cli.ObjectClient(ctx, u)
		if err != nil {
			return "", err
		}
		live, err := c.Get(ctx, u.GetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("the profile network (%s %s) does not exist; configure the provider before launching", u.GetKind(), u.GetName())
		}
		if err != nil {
			return "", err
		}
		if !owned(live, labels) || live.GetDeletionTimestamp() != nil {
			return "", fmt.Errorf("the profile network (%s %s) is not available for this profile", u.GetKind(), u.GetName())
		}
		if ready, _ := readiness(live); !ready {
			return "", fmt.Errorf("the profile network (%s %s) is not ready", u.GetKind(), u.GetName())
		}
		if u.GetKind() == vpc.RouteTable_Kind {
			routeID, _, _ = unstructured.NestedString(live.Object, "status", "atProvider", "id") //nolint:errcheck // readiness checked it
		}
	}
	return routeID, nil
}

func ptr[T any](v T) *T { return &v }
