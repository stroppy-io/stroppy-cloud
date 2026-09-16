package system

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestRuntime(t *testing.T) {
	schematest.Run(t, Runtime(), schematest.Cases{Valid: []map[string]any{{}, {"machines": map[string]any{"db-1": map[string]any{"cpu": int64(8), "memory_gb": int64(32), "public_ip": false}}}}})
}
