package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"
	"github.com/graphene-ci/pipeline/pkg/manifest"
)

// liveFixturePath keeps historical fixture identities while their files live
// beside the corresponding database cases or shared platform checks.
func liveFixturePath(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile("../live/tests/platform/migration/paths.json")
	if err != nil {
		t.Fatal(err)
	}
	var paths map[string]string
	if err := json.Unmarshal(raw, &paths); err != nil {
		t.Fatal(err)
	}
	path, ok := paths[name]
	if !ok || !filepath.IsLocal(path) {
		t.Fatalf("invalid live fixture mapping: %s -> %s", name, path)
	}
	return filepath.Join("../live", path)
}

// The Graphene door validates the reflected manifest, not our product schema.
// JSON values emitted by the server must pass both, including omitted zeros.
func TestOmittedFieldsFitGrapheneManifest(t *testing.T) {
	run := sampleRun()
	run.Observability = Observability{}
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"run", run},
		{"suite", Suite{SuiteRunID: "suite", Tenant: "acme", Cells: []SuiteCell{{ID: "one", RunSpec: run}}}},
		{"result", Result{}},
		{"quotas-unavailable", QuotasResult{ObservedAt: time.Now().UTC(), UnavailableReason: "permission_denied", Scope: "cloud:b1g", Quotas: []Quota{}}},
		{"quotas-available", QuotasResult{ObservedAt: time.Now().UTC(), Quotas: []Quota{}}},
		{"segment-result", SegmentResult{Name: "failed", Status: SegmentFailed, ExitCode: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := manifest.SchemaOf(reflect.TypeOf(tc.value), schemapb.ID("test", schemapb.SchemaName(tc.name), schemapb.Ver(1, 0, 0)))
			if err != nil {
				t.Fatal(err)
			}
			engine, err := schemapb.Compile(s)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			_, result, err := engine.Bake(value)
			if err != nil {
				t.Fatal(err)
			}
			if result.Blocking() {
				for _, issue := range result.GetErrors() {
					t.Errorf("%s: %s %s", issue.GetPath(), issue.GetCode(), issue.GetMessage())
				}
			}
		})
	}
}

func TestLiveFixturesFitProductSchemas(t *testing.T) {
	for _, tc := range []struct{ file, schema string }{
		{"verify-yandex.json", "spec.provider_verify@1"},
		{"verify-yandex.result.json", "spec.result.provider_verify@1"},
		{"verify-missing-secret.json", "spec.provider_verify@1"},
		{"quotas-yandex.json", "spec.quotas@1"},
		{"provider-yandex-zone-a.json", "spec.quotas@1"},
		{"quotas-yandex-unavailable.result.json", "spec.result.quotas@1"},
		{"run-yandex-noop.json", "spec.run@1"},
		{"run-yandex-postgres.json", "spec.run@1"},
		{"run-yandex-postgres.result.json", "spec.result.run@1"},
		{"postgres-disk-zone-a.json", "spec.run@1"},
		{"postgres-disk-zone-a.result.json", "spec.result.run@1"},
		{"postgres-host-metrics.json", "spec.run@1"},
		{"postgres-host-metrics.result.json", "spec.result.run@1"},
		{"suite-yandex-postgres-parallel.json", "spec.suite@1"},
		{"suite-postgres18-topologies.json", "spec.suite@1"},
		{"suite-postgres-versions.json", "spec.suite@1"},
		{"suite-postgres-versions-retry.json", "spec.suite@1"},
		{"suite-postgres-versions-final.json", "spec.suite@1"},
		{"suite-postgres-patroni.json", "spec.suite@1"},
		{"suite-native-exports.json", "spec.suite@1"},
		{"suite-native-tpcc-retry.json", "spec.suite@1"},
		{"suite-mysql84-single.json", "spec.suite@1"},
		{"suite-mysql-fixed.json", "spec.suite@1"},
		{"suite-mariadb-fixed.json", "spec.suite@1"},
		{"suite-mysql-family-final.json", "spec.suite@1"},
		{"suite-mysql-clusters.json", "spec.suite@1"},
		{"suite-mariadb-monitor-fixed.json", "spec.suite@1"},
		{"mariadb-galera-118.result.json", "spec.result.run@1"},
		{"mariadb-galera-114.result.json", "spec.result.run@1"},
		{"mariadb-galera-1011.result.json", "spec.result.run@1"},
		{"mariadb-single-118.result.json", "spec.result.run@1"},
		{"mariadb-single-114.result.json", "spec.result.run@1"},
		{"mariadb-single-1011.result.json", "spec.result.run@1"},
		{"mysql-gr-84.result.json", "spec.result.run@1"},
		{"mysql-gr-80.result.json", "spec.result.run@1"},
		{"suite-mysql84-single-fixed.json", "spec.suite@1"},
		{"mysql84-single.result.json", "spec.result.run@1"},
		{"mysql84-semi-sync.result.json", "spec.result.run@1"},
		{"mysql80-semi-sync.result.json", "spec.result.run@1"},
		{"mysql80-single.result.json", "spec.result.run@1"},
		{"mariadb118-single.result.json", "spec.result.run@1"},
		{"mariadb114-single.result.json", "spec.result.run@1"},
		{"mariadb1011-single.result.json", "spec.result.run@1"},
		{"maria114-single.result.json", "spec.result.run@1"},
		{"native-tpcb.result.json", "spec.result.run@1"},
		{"native-tpcc.result.json", "spec.result.run@1"},
		{"native-tpcc-contention.result.json", "spec.result.run@1"},
		{"pg15-patroni-ha.result.json", "spec.result.run@1"},
		{"pg16-patroni-ha.result.json", "spec.result.run@1"},
		{"pg17-patroni-ha.result.json", "spec.result.run@1"},
		{"pg18-patroni-ha.result.json", "spec.result.run@1"},
		{"pg15-single.result.json", "spec.result.run@1"},
		{"pg15-primary-replica.result.json", "spec.result.run@1"},
		{"pg15-pgbouncer.result.json", "spec.result.run@1"},
		{"pg16-single.result.json", "spec.result.run@1"},
		{"pg16-primary-replica.result.json", "spec.result.run@1"},
		{"pg16-pgbouncer.result.json", "spec.result.run@1"},
		{"pg17-single.result.json", "spec.result.run@1"},
		{"pg17-primary-replica.result.json", "spec.result.run@1"},
		{"pg17-pgbouncer.result.json", "spec.result.run@1"},
		{"pg18-single.result.json", "spec.result.run@1"},
		{"pg18-primary-replica.result.json", "spec.result.run@1"},
		{"pg18-pgbouncer.result.json", "spec.result.run@1"},
		{"run-yandex-keep.json", "spec.run@1"},
		{"run-yandex-keep.result.json", "spec.result.run@1"},
		{"suite-yandex-noop.json", "spec.suite@1"},
		{"run-yandex-missing-credentials.json", "spec.run@1"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(liveFixturePath(t, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			bake(t, tc.schema, value)
		})
	}
}

// TestStructuredLiveInputsFitProductSchema checks extracted public run inputs
// and unlaunched drafts after moving them alongside their test cases.
func TestStructuredLiveInputsFitProductSchema(t *testing.T) {
	files, err := filepath.Glob("../live/tests/*/*/*/*/*/input.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no structured live inputs found")
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			bake(t, "spec.run@1", value)
		})
	}
}

// SuiteResult currently has only the reflected Graphene output contract.
func TestLiveSuiteResultFitsGrapheneManifest(t *testing.T) {
	schema, err := manifest.SchemaOf(reflect.TypeOf(SuiteResult{}), schemapb.ID("test", "suite-result", schemapb.Ver(1, 0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := schemapb.Compile(schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"suite-yandex-noop.result.json",
		"suite-mysql-family-final.result.json",
		"suite-mariadb-monitor-fixed.result.json",
		"suite-mysql84-single-fixed.result.json",
		"suite-postgres18-topologies.result.json",
		"suite-postgres-versions-final.result.json",
		"suite-postgres-patroni.result.json",
		"suite-yandex-postgres-parallel.result.json",
		"suite-yandex-postgres-parallel-canceled.result.json",
		"suite-yandex-postgres-parallel-dockerhub.result.json",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(liveFixturePath(t, name))
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			_, result, err := engine.Bake(value)
			if err != nil {
				t.Fatal(err)
			}
			if result.Blocking() {
				t.Fatalf("live suite result violates Graphene output schema: %v", result.GetErrors())
			}
		})
	}
}
