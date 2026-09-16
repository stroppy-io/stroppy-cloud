package activities

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadPullProgress(t *testing.T) {
	for _, tc := range []struct {
		name, stream, want string
	}{
		{"success", "{\"status\":\"Downloading\"}\n{\"status\":\"Pull complete\"}\n", ""},
		{"daemon error after progress", "{\"status\":\"Downloading\"}\n{\"errorDetail\":{\"message\":\"failed to extract layer\"},\"error\":\"failed to extract layer\"}\n", "failed to extract layer"},
		{"legacy error", `{"error":"access denied"}`, "access denied"},
		{"truncated response", `{"status":`, "decode docker progress"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := readPullProgress(context.Background(), strings.NewReader(tc.stream))
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}
