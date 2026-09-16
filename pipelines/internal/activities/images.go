package activities

import (
	"context"

	"github.com/graphene-ci/pipeline/pkg/workerapi"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/cloud"
)

const NameResolveYandexImages = "stroppy.provider.resolve-yandex-images"

type ResolveYandexImagesRequest struct {
	CredentialsSecret string   `json:"credentials_secret"`
	Images            []string `json:"images"`
}

type ResolveYandexImagesResult struct {
	Images map[string]string `json:"images"`
}

func ResolveYandexImages(ctx context.Context, req ResolveYandexImagesRequest) (ResolveYandexImagesResult, error) {
	credentials, err := workerapi.GetSecret(ctx, req.CredentialsSecret)
	if err != nil {
		return ResolveYandexImagesResult{}, err
	}
	images, err := (cloud.Yandex{}).ResolveImages(ctx, credentials, req.Images)
	return ResolveYandexImagesResult{Images: images}, err
}
