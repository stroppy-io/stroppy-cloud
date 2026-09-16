package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/quotamanager/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/resourcemanager/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Yandex uses the Yandex Cloud SDK with a service-account key.
//
// doc: https://yandex.cloud/en/docs/quota-manager/api-ref/grpc/QuotaLimit/list
// doc: https://yandex.cloud/en/docs/compute/concepts/limits
type Yandex struct{}

// yandexSettings is the part of provider.yandex.settings@1 the probes use.
type yandexSettings struct {
	CloudID  string `json:"cloud_id"`
	FolderID string `json:"folder_id"`
	Zone     string `json:"zone"`
}

// yandexCredentials is provider.yandex.credentials@1.
type yandexCredentials struct {
	SAKeyJSON string `json:"sa_key_json"`
}

// quotaIDs are the compute quotas a run consumes; their ids are the
// Quota Manager ids of the compute service.
//
// doc: https://yandex.cloud/en/docs/compute/concepts/limits#compute-quotas
var yandexQuotaIDs = []struct{ id, unit string }{
	{"compute.instances.count", "instances"},
	{"compute.instanceCores.count", "cores"},
	{"compute.instanceMemory.size", "bytes"},
	{"compute.ssdDisks.size", "bytes"},
	{"compute.hddDisks.size", "bytes"},
	{"compute.ssdNonReplicatedDisks.size", "bytes"},
	{"compute.disks.count", "disks"},
	{"vpc.networks.count", "networks"},
	{"vpc.subnets.count", "subnets"},
	{"vpc.securityGroups.count", "security-groups"},
	{"vpc.externalStaticAddresses.count", "addresses"},
}

func (Yandex) sdk(ctx context.Context, credentials string) (*ycsdk.SDK, error) {
	var c yandexCredentials
	if err := json.Unmarshal([]byte(credentials), &c); err != nil {
		return nil, fmt.Errorf("yandex credentials: %w", err)
	}
	key, err := iamkey.ReadFromJSONBytes([]byte(c.SAKeyJSON))
	if err != nil {
		return nil, fmt.Errorf("yandex sa key: %w", err)
	}
	creds, err := ycsdk.ServiceAccountKey(key)
	if err != nil {
		return nil, fmt.Errorf("yandex sa key: %w", err)
	}
	return ycsdk.Build(ctx, ycsdk.Config{Credentials: creds})
}

// Verify authenticates with the key and checks the rights a run needs:
// read the folder, list compute instances, list VPC networks. With dryRun
// false it additionally lists images (the run's boot images) — still
// read-only; nothing billable is ever created here.
func (y Yandex) Verify(ctx context.Context, settings json.RawMessage, credentials string, dryRun bool) (spec.ProviderVerifyResult, error) {
	var st yandexSettings
	if err := json.Unmarshal(settings, &st); err != nil {
		return spec.ProviderVerifyResult{}, fmt.Errorf("yandex settings: %w", err)
	}
	if st.FolderID == "" {
		return spec.ProviderVerifyResult{OK: false, Error: "folder_id is required"}, nil
	}
	sdk, err := y.sdk(ctx, credentials)
	if err != nil {
		return spec.ProviderVerifyResult{OK: false, Error: err.Error()}, nil
	}
	defer sdk.Shutdown(ctx) //nolint:errcheck // best-effort connection close
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	res := spec.ProviderVerifyResult{Scope: "folder:" + st.FolderID}
	folder, err := sdk.ResourceManager().Folder().Get(ctx, &resourcemanager.GetFolderRequest{FolderId: st.FolderID})
	res.Permissions = append(res.Permissions, permission("resource-manager.folders.get", err))
	if err != nil {
		res.Error = "folder: " + err.Error()
		return res, nil
	}
	res.AccountID = folder.GetCloudId()
	if st.CloudID != "" && st.CloudID != folder.GetCloudId() {
		res.Error = fmt.Sprintf("folder %s belongs to cloud %s, not %s", st.FolderID, folder.GetCloudId(), st.CloudID)
		return res, nil
	}
	_, err = sdk.Compute().Instance().List(ctx, &compute.ListInstancesRequest{FolderId: st.FolderID, PageSize: 1})
	res.Permissions = append(res.Permissions, permission("compute.instances.list", err))
	_, err2 := sdk.VPC().Network().List(ctx, &vpc.ListNetworksRequest{FolderId: st.FolderID, PageSize: 1})
	res.Permissions = append(res.Permissions, permission("vpc.networks.list", err2))
	if !dryRun {
		_, err3 := sdk.Compute().Image().List(ctx, &compute.ListImagesRequest{FolderId: "standard-images", PageSize: 1})
		res.Permissions = append(res.Permissions, permission("compute.images.list", err3))
	}
	var denied []string
	for _, p := range res.Permissions {
		if !p.Granted {
			denied = append(denied, p.Name)
		}
	}
	res.OK = len(denied) == 0
	if !res.OK {
		res.Error = "missing rights: " + strings.Join(denied, ", ")
	}
	return res, nil
}

// Quotas reads the compute/vpc quota limits and usage of the folder's
// cloud through Quota Manager.
func (y Yandex) Quotas(ctx context.Context, settings json.RawMessage, credentials, _ string) (spec.QuotasResult, error) {
	var st yandexSettings
	if err := json.Unmarshal(settings, &st); err != nil {
		return spec.QuotasResult{}, fmt.Errorf("yandex settings: %w", err)
	}
	sdk, err := y.sdk(ctx, credentials)
	if err != nil {
		return spec.QuotasResult{}, err
	}
	defer sdk.Shutdown(ctx) //nolint:errcheck // best-effort connection close
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cloudID := st.CloudID
	if cloudID == "" {
		folder, err := sdk.ResourceManager().Folder().Get(ctx, &resourcemanager.GetFolderRequest{FolderId: st.FolderID})
		if err != nil {
			return spec.QuotasResult{}, fmt.Errorf("folder %s: %w", st.FolderID, err)
		}
		cloudID = folder.GetCloudId()
	}
	return readYandexQuotas(ctx, sdk.QuotaManager().QuotaLimit(), cloudID)
}

type yandexQuotaClient interface {
	List(context.Context, *quotamanager.ListQuotaLimitsRequest, ...grpc.CallOption) (*quotamanager.ListQuotaLimitsResponse, error)
}

func readYandexQuotas(ctx context.Context, client yandexQuotaClient, cloudID string) (spec.QuotasResult, error) {
	resource := &quotamanager.Resource{Id: cloudID, Type: "resource-manager.cloud"}
	out := spec.QuotasResult{ObservedAt: time.Now().UTC(), Scope: "cloud:" + cloudID, Quotas: []spec.Quota{}}
	for _, svc := range []string{"compute", "vpc"} {
		req := &quotamanager.ListQuotaLimitsRequest{Resource: resource, Service: svc}
		for {
			page, err := client.List(ctx, req)
			if status.Code(err) == codes.PermissionDenied {
				// Cloud-level quota visibility is optional for a folder-scoped
				// account. Discard partial pages: an incomplete snapshot must
				// not look like a successful capacity check.
				out.Quotas = []spec.Quota{}
				out.UnavailableReason = "permission_denied"
				return out, nil
			}
			if err != nil {
				return spec.QuotasResult{}, fmt.Errorf("quota limits of %s: %w", svc, err)
			}
			for _, q := range page.GetQuotaLimits() {
				unit, wanted := yandexUnit(q.GetQuotaId())
				if !wanted {
					continue
				}
				out.Quotas = append(out.Quotas, spec.Quota{
					Name:  q.GetQuotaId(),
					Limit: q.GetLimit().GetValue(),
					Used:  q.GetUsage().GetValue(),
					Unit:  unit,
				})
			}
			if page.GetNextPageToken() == "" {
				break
			}
			req.PageToken = page.GetNextPageToken()
		}
	}
	return out, nil
}

func yandexUnit(id string) (string, bool) {
	for _, q := range yandexQuotaIDs {
		if q.id == id {
			return q.unit, true
		}
	}
	return "", false
}
