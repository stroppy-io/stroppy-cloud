package dbparams

import (
	"testing"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestMPP(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schema   func() *schemapb.Schema
		versions []string
		oldest   string
	}{
		{"greenplum", Greenplum, []string{"7.1.0", "6.27.1"}, "6.27.1"},
		{"greengage", Greengage, []string{"8.0.0-alpha.1", "7.5.1", "6.32.0"}, "6.32.0"},
		{"warehousepg", Warehousepg, []string{"7.6.1", "6.27.7"}, "6.27.7"},
		{"opengpdb", Opengpdb, []string{"6.30.2"}, "6.30.2"},
		{"cloudberry", Cloudberry, []string{"2.1.0"}, "2.1.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			valid := []map[string]any{
				{"version": tc.oldest},
				{"version": tc.oldest, "tuning": "stock", "segment_hosts": int64(32), "segments_per_host": int64(64), "optimizer": "postgres"},
			}
			for _, v := range tc.versions {
				valid = append(valid, map[string]any{"version": v, "segment_hosts": int64(2), "segments_per_host": int64(4)})
			}
			schematest.Run(t, tc.schema(), schematest.Cases{
				Valid: valid,
				Invalid: []schematest.Invalid{
					{Value: map[string]any{"segment_hosts": int64(0)}, Code: "GTE_VIOLATED", Path: "segment_hosts"},
					{Value: map[string]any{"segment_hosts": int64(33)}, Code: "LTE_VIOLATED", Path: "segment_hosts"},
					{Value: map[string]any{"segments_per_host": int64(-1)}, Code: "GTE_VIOLATED", Path: "segments_per_host"},
					{Value: map[string]any{"segments_per_host": int64(65)}, Code: "LTE_VIOLATED", Path: "segments_per_host"},
					{Value: map[string]any{"optimizer": "both"}, Code: "CHOICE_NOT_ALLOWED", Path: "optimizer"},
					{Value: map[string]any{"version": "5.0"}, Code: "CHOICE_NOT_ALLOWED", Path: "version"},
					{Value: map[string]any{"mirrors": int64(1)}, Code: "UNKNOWN_FIELD", Path: "mirrors"},
				},
			})
		})
	}
}
