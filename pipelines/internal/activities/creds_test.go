package activities

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderCheckRejectsForeignConfiguration(t *testing.T) {
	_, err := EnsureProviderConfig(context.Background(), EnsureProviderConfigRequest{Name: "another-tenant"})
	require.ErrorContains(t, err, "does not match")
}
