package cloud

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"google.golang.org/grpc"
)

var yandexImageID = regexp.MustCompile(`^[a-z0-9]{20}$`)

// IsYandexImageID distinguishes pinned IDs from image families. A family can
// be qualified as folder-id/family, avoiding ambiguity with an opaque ID.
func IsYandexImageID(image string) bool { return yandexImageID.MatchString(image) }

// ResolveImages pins each family to one image for this run. Temporal records
// the activity result, so every machine and replay uses the same image.
// doc: https://yandex.cloud/en/docs/compute/api-ref/grpc/Image/getLatestByFamily
func (y Yandex) ResolveImages(ctx context.Context, credentials string, images []string) (map[string]string, error) {
	sdk, err := y.sdk(ctx, credentials)
	if err != nil {
		return nil, err
	}
	defer sdk.Shutdown(ctx) //nolint:errcheck // best-effort connection close
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	return resolveYandexImages(ctx, sdk.Compute().Image(), images)
}

type yandexImageClient interface {
	GetLatestByFamily(context.Context, *compute.GetImageLatestByFamilyRequest, ...grpc.CallOption) (*compute.Image, error)
}

func resolveYandexImages(ctx context.Context, client yandexImageClient, images []string) (map[string]string, error) {
	out := make(map[string]string, len(images))
	for _, image := range images {
		if _, ok := out[image]; ok {
			continue
		}
		if IsYandexImageID(image) {
			out[image] = image
			continue
		}
		folder, family := "standard-images", image
		if f, name, qualified := strings.Cut(image, "/"); qualified {
			folder, family = f, name
		}
		if folder == "" || family == "" {
			return nil, fmt.Errorf("image family %q is empty", image)
		}
		resolved, err := client.GetLatestByFamily(ctx, &compute.GetImageLatestByFamilyRequest{FolderId: folder, Family: family})
		if err != nil {
			return nil, fmt.Errorf("image family %s/%s: %w", folder, family, err)
		}
		if !IsYandexImageID(resolved.GetId()) {
			return nil, fmt.Errorf("image family %s/%s returned no valid image ID", folder, family)
		}
		out[image] = resolved.GetId()
	}
	return out, nil
}
