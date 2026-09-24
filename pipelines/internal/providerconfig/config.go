// Package providerconfig manages persistent cloud-profile configuration through
// the installation's Kubernetes adapter. Its lifetime is the provider profile,
// independent of workload runs and their cleanup.
package providerconfig

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/wire"
	"github.com/graphene-ci/pipeline/pkg/workerapi"
	"go.temporal.io/sdk/activity"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const (
	ownerLabel     = "stroppy.io/provider-profile"
	namespaceLabel = "stroppy.io/graphene-namespace"
)

type adapter interface {
	ObjectClient(context.Context, *unstructured.Unstructured) (dynamic.ResourceInterface, error)
}

func identity(p spec.ProviderConfig, ns string) (name string, labels map[string]string, err error) {
	id, err := uuid.Parse(p.ProfileID)
	if err != nil || id.String() != p.ProfileID || ns == "" {
		return "", nil, fmt.Errorf("provider configuration requires a canonical profile UUID and worker namespace")
	}
	if p.CredentialsSecret != "provider-"+p.ProfileID && !strings.HasPrefix(p.CredentialsSecret, "provider-"+p.ProfileID+"-") {
		return "", nil, fmt.Errorf("credential reference does not belong to the provider profile")
	}
	if p.Provider != spec.ProviderYandex && p.Provider != spec.ProviderAWS {
		return "", nil, fmt.Errorf("unsupported provider")
	}
	name = spec.ProviderConfigName(ns, p.ProfileID)
	return name, map[string]string{"app.kubernetes.io/managed-by": "stroppy-cloud", ownerLabel: p.ProfileID, namespaceLabel: ns}, nil
}

// Configure runs only on the Graphene worker. No secret value is returned.
func Configure(ctx context.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
	name, labels, err := identity(p, os.Getenv(wire.EnvNamespace))
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	cli := k8slib.NewClientInCluster()
	switch p.Action {
	case "delete":
		err = remove(ctx, cli, p.Provider, name, labels)
	case "ensure":
		var raw string
		raw, err = workerapi.GetSecret(ctx, p.CredentialsSecret)
		if err == nil {
			err = ensure(ctx, cli, p, name, labels, raw)
		}
	default:
		err = fmt.Errorf("action must be ensure or delete")
	}
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	return spec.ProviderVerifyResult{OK: true}, nil
}

func configObject(kind spec.ProviderKind, name string) *unstructured.Unstructured {
	version := "yandex-cloud.jet.crossplane.io/v1beta1"
	if kind == spec.ProviderAWS {
		version = "aws.upbound.io/v1beta1"
	}
	return object(version, "ProviderConfig", "", name)
}

func object(version, kind, ns, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": version, "kind": kind, "metadata": map[string]any{"name": name}}}
	if ns != "" {
		u.SetNamespace(ns)
	}
	return u
}

func owned(u *unstructured.Unstructured, labels map[string]string) bool {
	for k, v := range labels {
		if u.GetLabels()[k] != v {
			return false
		}
	}
	return true
}

func upsert(ctx context.Context, cli adapter, u *unstructured.Unstructured, labels map[string]string) error {
	c, err := cli.ObjectClient(ctx, u)
	if err != nil {
		return err
	}
	old, err := c.Get(ctx, u.GetName(), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err == nil {
		if !owned(old, labels) {
			return fmt.Errorf("refusing to overwrite configuration owned by another profile")
		}
		if old.GetDeletionTimestamp() != nil {
			return fmt.Errorf("provider configuration is being deleted")
		}
		u.SetResourceVersion(old.GetResourceVersion())
		// Preserve controller-owned lifecycle metadata during credential rotation.
		u.SetFinalizers(old.GetFinalizers())
		u.SetAnnotations(old.GetAnnotations())
		u.SetOwnerReferences(old.GetOwnerReferences())
	}
	u.SetLabels(labels)
	if apierrors.IsNotFound(err) {
		_, err = c.Create(ctx, u, metav1.CreateOptions{})
	} else {
		_, err = c.Update(ctx, u, metav1.UpdateOptions{})
	}
	// Kubernetes validation errors can echo rejected Secret data.
	if err != nil && u.GetKind() == "Secret" {
		return fmt.Errorf("provider credential Secret write failed (%s)", apierrors.ReasonForError(err))
	}
	return err
}

func ensure(ctx context.Context, cli adapter, p spec.ProviderConfig, name string, labels map[string]string, raw string) error {
	if err := unused(ctx, cli, p.Provider, name); err != nil {
		return err
	}
	data, pc, err := providerObjects(request{Provider: p.Provider, Name: name, Settings: p.Settings}, raw)
	if err != nil {
		return fmt.Errorf("invalid provider credential or settings payload")
	}
	secret := object("v1", "Secret", crossplaneNamespace, name)
	encoded := map[string]any{}
	for k, v := range data {
		encoded[k] = base64.StdEncoding.EncodeToString(v)
	}
	secret.Object["data"] = encoded
	secret.Object["type"] = "Opaque"
	// Check both owners before changing either resource.
	for _, u := range []*unstructured.Unstructured{pc, secret} {
		c, e := cli.ObjectClient(ctx, u)
		if e != nil {
			return e
		}
		old, e := c.Get(ctx, name, metav1.GetOptions{})
		if e != nil && !apierrors.IsNotFound(e) {
			return e
		}
		if e == nil && !owned(old, labels) {
			return fmt.Errorf("provider configuration ownership conflict")
		}
		if e == nil && old.GetDeletionTimestamp() != nil {
			return fmt.Errorf("provider configuration is being deleted")
		}
	}
	if err := upsert(ctx, cli, secret, labels); err != nil {
		return err
	}
	return upsert(ctx, cli, pc, labels)
}

// Check verifies the pre-created configuration without modifying it.
func Check(ctx context.Context, p spec.ProviderConfig) (spec.ProviderVerifyResult, error) {
	name, labels, err := identity(p, os.Getenv(wire.EnvNamespace))
	if err != nil {
		return spec.ProviderVerifyResult{}, err
	}
	cli := k8slib.NewClientInCluster()
	for _, u := range []*unstructured.Unstructured{configObject(p.Provider, name), object("v1", "Secret", crossplaneNamespace, name)} {
		c, e := cli.ObjectClient(ctx, u)
		if e != nil {
			return spec.ProviderVerifyResult{}, e
		}
		live, e := c.Get(ctx, name, metav1.GetOptions{})
		if e != nil {
			return spec.ProviderVerifyResult{}, e
		}
		if !owned(live, labels) || live.GetDeletionTimestamp() != nil {
			return spec.ProviderVerifyResult{}, fmt.Errorf("provider configuration is not available for this profile")
		}
	}
	return spec.ProviderVerifyResult{OK: true}, nil
}

func remove(ctx context.Context, cli adapter, kind spec.ProviderKind, name string, labels map[string]string) error {
	if err := unused(ctx, cli, kind, name); err != nil {
		return err
	}
	// Wait for ProviderConfig finalizers BEFORE deleting its credentials.
	for _, u := range []*unstructured.Unstructured{configObject(kind, name), object("v1", "Secret", crossplaneNamespace, name)} {
		if err := deleteOwned(ctx, cli, u, labels); err != nil {
			return err
		}
	}
	return nil
}

func deleteOwned(ctx context.Context, cli adapter, u *unstructured.Unstructured, labels map[string]string) error {
	c, err := cli.ObjectClient(ctx, u)
	if err != nil {
		return err
	}
	live, err := c.Get(ctx, u.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !owned(live, labels) {
		return fmt.Errorf("refusing to delete configuration owned by another profile")
	}
	uid := live.GetUID()
	rv := live.GetResourceVersion()
	if err = c.Delete(ctx, u.GetName(), metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &rv}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		live, err = c.Get(ctx, u.GetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if live.GetUID() != uid {
			return fmt.Errorf("configuration was replaced during deletion")
		}
		if activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, "waiting for provider configuration deletion")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Scan actual managed objects as well as usages: cached server state and usage
// counts may lag, and a deleting object still requires provider credentials.
func unused(ctx context.Context, cli adapter, kind spec.ProviderKind, name string) error {
	kinds := managedKinds(kind)
	for _, k := range kinds {
		c, err := cli.ObjectClient(ctx, object(k.version, k.kind, "", ""))
		if err != nil {
			return err
		}
		list, err := c.List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, u := range list.Items {
			ref, _, err := unstructured.NestedString(u.Object, "spec", "providerConfigRef", "name")
			if err != nil {
				return fmt.Errorf("invalid provider reference: %w", err)
			}
			if k.kind == "ProviderConfigUsage" {
				ref, _, err = unstructured.NestedString(u.Object, "providerConfigRef", "name")
				if err != nil {
					return fmt.Errorf("invalid provider usage: %w", err)
				}
			}
			if ref == name {
				return fmt.Errorf("provider configuration is still in use by %s/%s", k.kind, u.GetName())
			}
		}
	}
	return nil
}

func managedKinds(kind spec.ProviderKind) []struct{ version, kind string } {
	kinds := []struct{ version, kind string }{}
	if kind == spec.ProviderYandex {
		for _, k := range []string{"Instance", "Disk"} {
			kinds = append(kinds, struct{ version, kind string }{"compute.yandex-cloud.jet.crossplane.io/v1alpha1", k})
		}
		for _, k := range []string{"Network", "Subnet", "SecurityGroup", "Gateway", "RouteTable"} {
			kinds = append(kinds, struct{ version, kind string }{"vpc.yandex-cloud.jet.crossplane.io/v1alpha1", k})
		}
		for _, k := range []string{"DatabaseServerless", "DatabaseDedicated"} {
			kinds = append(kinds, struct{ version, kind string }{"ydb.yandex-cloud.jet.crossplane.io/v1alpha1", k})
		}
	} else {
		for _, k := range []string{"VPC", "Subnet", "InternetGateway", "RouteTable", "Route", "RouteTableAssociation", "SecurityGroup", "SecurityGroupRule", "Instance"} {
			kinds = append(kinds, struct{ version, kind string }{"ec2.aws.upbound.io/v1beta1", k})
		}
	}
	pc := configObject(kind, "")
	kinds = append(kinds, struct{ version, kind string }{pc.GetAPIVersion(), "ProviderConfigUsage"})

	return kinds
}
