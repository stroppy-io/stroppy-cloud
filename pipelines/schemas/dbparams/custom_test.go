package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestCustom(t *testing.T) {
	schematest.Run(t, Custom(), schematest.Cases{
		Valid: []map[string]any{
			{"image": "ghcr.io/acme/mydb:1.4.2"},
			{
				"image": "postgres:17", "protocol": "pg", "port": int64(5432), "nodes": int64(3),
				"env": map[string]any{"POSTGRES_USER": "stroppy", "PEERS": "${ips:role:db}"},
				"cmd": []any{"postgres", "-c", "fsync=off"}, "data_dir": "/var/lib/postgresql/data",
				"healthcheck": "pg_isready -U stroppy",
			},
			{"image": "ydbplatform/local-ydb:24.4", "protocol": "ydb_grpc", "port": int64(2136), "database": "/local"},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{}, Code: "REQUIRED_MISSING", Path: "image"},
			{Value: map[string]any{"image": "x", "nodes": int64(0)}, Code: "GTE_VIOLATED", Path: "nodes"},
			{Value: map[string]any{"image": "x", "protocol": "redis"}, Code: "CHOICE_NOT_ALLOWED", Path: "protocol"},
			{Value: map[string]any{"image": "x", "protocol": "ydb_grpc", "database": "stroppy"}, Code: "RULE_VIOLATED", Path: ""},
		},
	})
}
