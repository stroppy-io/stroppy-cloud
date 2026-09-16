package provision

import (
	"fmt"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/pipeline"
	ydb "github.com/yandex-cloud/crossplane-provider-yc/apis/cluster/ydb/v1alpha1"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Managed databases belong to the run tree, below their zonal subnets.
// Cleanup therefore removes the database before any subnet or security group.
func yandexManagedEndpoint(ctx pipeline.Context, client *k8slib.Client, run spec.Run, settings YandexSettings, names Names, subnets []yandexSubnet, groupID string, parent pipeline.Handle) (string, error) {
	m := run.ManagedYDB
	name := names.Prefix() + "-ydb"
	switch m.Type {
	case "serverless":
		handle := k8slib.Resource(ctx, client, name, &ydb.DatabaseServerless{Spec: ydb.DatabaseServerlessSpec{
			ResourceSpec: providerRef(run.Provider.ProviderConfigName),
			ForProvider: ydb.DatabaseServerlessParameters{
				FolderID: ptr(settings.FolderID), Name: ptr(name), LocationID: ptr(m.LocationID), Labels: labels(run, nil), DeletionProtection: ptr(false),
				ServerlessDatabase: []ydb.ServerlessDatabaseParameters{{
					EnableThrottlingRcuLimit: ptr(m.ThrottlingRCULimit > 0), ThrottlingRcuLimit: ptr(float64(m.ThrottlingRCULimit)),
					ProvisionedRcuLimit: ptr(float64(m.ProvisionedRCULimit)), StorageSizeLimit: ptr(float64(m.StorageSizeLimitGB)),
				}},
			},
		}}, k8slib.WithReady(yandexServerlessReady), k8slib.WithTimeout[ydb.DatabaseServerless](30*time.Minute), k8slib.WithResourceOption[ydb.DatabaseServerless](pipeline.Parent(parent)))
		live, err := handle.TryReady(ctx)
		if err != nil {
			return "", err
		}
		return *live.Status.AtProvider.YdbFullEndpoint, nil
	case "dedicated":
		if groupID == "" {
			return "", fmt.Errorf("provision/yandex: managed YDB security group has no observed id")
		}
		refs := make([]xpv1.Reference, 0, len(subnets))
		for _, sub := range subnets {
			refs = append(refs, xpv1.Reference{Name: sub.Name})
		}
		handle := k8slib.Resource(ctx, client, name, &ydb.DatabaseDedicated{Spec: ydb.DatabaseDedicatedSpec{
			ResourceSpec: providerRef(run.Provider.ProviderConfigName),
			ForProvider: ydb.DatabaseDedicatedParameters{
				FolderID: ptr(settings.FolderID), Name: ptr(name), LocationID: ptr(m.LocationID), Labels: labels(run, nil), DeletionProtection: ptr(false),
				NetworkIDRef: &xpv1.Reference{Name: names.Network()}, SubnetIdsRefs: refs, SecurityGroupIds: strPtrs(groupID),
				AssignPublicIps: ptr(m.AssignPublicIPs), ResourcePresetID: ptr(m.ResourcePresetID),
				ScalePolicy:   []ydb.ScalePolicyParameters{{FixedScale: []ydb.FixedScaleParameters{{Size: ptr(float64(m.NodeCount))}}}},
				StorageConfig: []ydb.StorageConfigParameters{{GroupCount: ptr(float64(m.StorageGroups)), StorageTypeID: ptr(m.StorageType)}},
			},
		}}, k8slib.WithReady(yandexDedicatedReady), k8slib.WithTimeout[ydb.DatabaseDedicated](30*time.Minute), k8slib.WithResourceOption[ydb.DatabaseDedicated](pipeline.Parent(parent)))
		live, err := handle.TryReady(ctx)
		if err != nil {
			return "", err
		}
		return *live.Status.AtProvider.YdbFullEndpoint, nil
	default:
		return "", fmt.Errorf("provision/yandex: unsupported managed YDB type %q", m.Type)
	}
}

func yandexServerlessReady(live *ydb.DatabaseServerless) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady)) && live.Status.AtProvider.YdbFullEndpoint != nil && *live.Status.AtProvider.YdbFullEndpoint != ""
}

func yandexDedicatedReady(live *ydb.DatabaseDedicated) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady)) && live.Status.AtProvider.YdbFullEndpoint != nil && *live.Status.AtProvider.YdbFullEndpoint != ""
}
