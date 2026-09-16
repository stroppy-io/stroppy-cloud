package system

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestMachine(t *testing.T) {
	schematest.Run(t, Machine(), schematest.Cases{Valid: []map[string]any{{}, {"cpu": int64(8), "memory_gb": int64(32), "public_ip": false, "boot_disk": map[string]any{"gb": int64(100), "type": "network-ssd"}}}})
}
