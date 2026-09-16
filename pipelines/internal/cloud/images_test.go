package cloud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type imageLatestFunc func(*compute.GetImageLatestByFamilyRequest) (*compute.Image, error)

func (f imageLatestFunc) GetLatestByFamily(_ context.Context, req *compute.GetImageLatestByFamilyRequest, _ ...grpc.CallOption) (*compute.Image, error) {
	return f(req)
}

func TestResolveYandexImagesPinsFamiliesOnce(t *testing.T) {
	const pinned = "fd8d6s0blceqbto92ss8"
	var requests []*compute.GetImageLatestByFamilyRequest
	client := imageLatestFunc(func(req *compute.GetImageLatestByFamilyRequest) (*compute.Image, error) {
		requests = append(requests, req)
		return &compute.Image{Id: pinned}, nil
	})
	resolved, err := resolveYandexImages(context.Background(), client, []string{pinned, "ubuntu-2404-lts", "ubuntu-2404-lts", "custom-folder/private-family"})
	require.NoError(t, err)
	require.Len(t, requests, 2)
	require.Equal(t, "standard-images", requests[0].FolderId)
	require.Equal(t, "ubuntu-2404-lts", requests[0].Family)
	require.Equal(t, "custom-folder", requests[1].FolderId)
	require.Equal(t, "private-family", requests[1].Family)
	require.Equal(t, map[string]string{pinned: pinned, "ubuntu-2404-lts": pinned, "custom-folder/private-family": pinned}, resolved)
}

func TestResolveYandexImagesDoesNotHideErrors(t *testing.T) {
	client := imageLatestFunc(func(*compute.GetImageLatestByFamilyRequest) (*compute.Image, error) {
		return nil, status.Error(codes.PermissionDenied, "denied")
	})
	_, err := resolveYandexImages(context.Background(), client, []string{"ubuntu-2404-lts"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	empty := imageLatestFunc(func(*compute.GetImageLatestByFamilyRequest) (*compute.Image, error) { return &compute.Image{}, nil })
	_, err = resolveYandexImages(context.Background(), empty, []string{"ubuntu-2404-lts"})
	require.ErrorContains(t, err, "no valid image ID")
}
