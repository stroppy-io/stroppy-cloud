package pipelines

import (
	"context"
	"fmt"
	"time"

	workerplanev1 "github.com/graphene-ci/pipeline/pkg/proto/workerplane/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
)

// ManifestAPI is the worker-plane gRPC half of the same Graphene door used by
// the SDK CLI. Management operations continue to use Connect.
func publishManifest(ctx context.Context, cfg *graphene.Config, namespace, image string, raw []byte) error {
	creds := credentials.NewTLS(nil)
	if cfg.Insecure {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(cfg.Address,
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(publishAuth{token: cfg.Token, namespace: namespace, insecure: cfg.Insecure}),
	)
	if err != nil {
		return fmt.Errorf("manifest connection: %w", err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err = workerplanev1.NewManifestAPIClient(conn).PublishManifest(ctx, &workerplanev1.PublishManifestRequest{
		Manifest: raw, Image: image,
	})
	if err != nil {
		return fmt.Errorf("publish manifest: %w", err)
	}
	return nil
}

type publishAuth struct {
	token, namespace string
	insecure         bool
}

func (a publishAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + a.token, graphene.NamespaceHeader: a.namespace}, nil
}

func (a publishAuth) RequireTransportSecurity() bool { return !a.insecure }
