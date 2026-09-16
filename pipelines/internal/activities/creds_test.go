package activities

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestKubeConfigUsesRunSecret(t *testing.T) {
	// A worker's host cluster need not be the cluster named by its run.
	t.Setenv("KUBERNETES_SERVICE_HOST", "host-cluster.example")
	t.Setenv("KUBERNETES_SERVICE_PORT", "443")
	const kubeconfig = `apiVersion: v1
kind: Config
clusters:
  - name: target
    cluster:
      server: https://target-cluster.example
contexts:
  - name: target
    context:
      cluster: target
      user: operator
current-context: target
users:
  - name: operator
    user:
      token: test-credential
`
	ctx := context.Background()
	called := false
	cfg, err := kubeConfig(ctx, func(got context.Context, name string) (string, error) {
		called = true
		if got != ctx || name != KubeconfigSecret {
			t.Fatalf("unexpected secret lookup: context=%v name=%q", got, name)
		}
		return kubeconfig, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called || cfg.Host != "https://target-cluster.example" || cfg.BearerToken != "test-credential" {
		t.Fatal("provider configuration must use the run's cluster and credentials")
	}
}

func TestKubeConfigSecretFailure(t *testing.T) {
	want := errors.New("secret is not configured")
	cfg, err := kubeConfig(context.Background(), func(context.Context, string) (string, error) {
		return "", want
	})
	if cfg != nil || !errors.Is(err, want) || !strings.Contains(err.Error(), KubeconfigSecret) {
		t.Fatalf("expected contextual secret error, got %v", err)
	}
}

func TestKubeConfigRejectsInvalidSecret(t *testing.T) {
	cfg, err := kubeConfig(context.Background(), func(context.Context, string) (string, error) {
		return "not a kubeconfig", nil
	})
	if cfg != nil || err == nil || !strings.Contains(err.Error(), KubeconfigSecret) {
		t.Fatalf("expected contextual kubeconfig error, got %v", err)
	}
}
