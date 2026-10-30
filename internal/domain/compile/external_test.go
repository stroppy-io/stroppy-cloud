package compile_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestExternalDSNPreservedForEveryProtocol(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	cases := []struct {
		protocol    catalog.Protocol
		dsn, driver string
	}{
		{catalog.ProtoPg, "postgres://test:example@db.internal:5432/bench", "postgres"},
		{catalog.ProtoMySQL, "test:example@tcp(db.internal:3306)/bench", "mysql"},
		{catalog.ProtoPicodata, "postgres://admin:example@db.internal:4327", "picodata"},
		{catalog.ProtoCockroach, "postgres://test:example@db.internal:26257/bench", "crdb"},
		{catalog.ProtoYDBGrpc, "grpc://db.internal:2136/Root/bench", "ydb"},
		{catalog.ProtoYDBGrpcs, "grpcs://db.internal:2135/Root/bench", "ydb"},
	}
	for _, tc := range cases {
		t.Run(string(tc.protocol), func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"protocol": string(tc.protocol), "dsn": tc.dsn})
			database, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: catalog.External, Version: "external", Params: raw})
			if err != nil {
				t.Fatal(err)
			}
			workload, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.3.0", Protocol: tc.protocol, Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)}})
			if err != nil {
				t.Fatal(err)
			}
			out, err := compile.Compile(ctx, reg, compile.Input{RunID: uuid.New(), Tenant: "test", Database: database, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: workload, WorkloadBaked: baked, Sizes: map[string]library.RoleSize{"runner": {Size: "S"}}, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-a","network":{"kind":"create"}}`), CredentialsSecret: "yc", Catalog: cat})
			if err != nil {
				t.Fatal(err)
			}
			if out.Spec.Workload.URL != tc.dsn || out.Spec.Workload.DriverType != tc.driver {
				t.Fatal("external DSN or driver replaced with generated endpoint")
			}
			if len(out.Spec.Machines) != 1 || out.Spec.Machines[0].Role != "runner" {
				t.Fatal("external test must only provision its runner")
			}
			exporter := ""
			for _, c := range out.Spec.Containers {
				if c.Role != "runner" {
					t.Fatal("external database component declared")
				}
				if c.Kind == spec.ContainerKindExporter && strings.HasSuffix(c.Name, "-postgres-exporter") && c.Env["DATA_SOURCE_NAME"] == tc.dsn {
					exporter = "postgres"
				}
				if c.Kind == spec.ContainerKindExporter && strings.HasSuffix(c.Name, "-mysqld-exporter") &&
					slices.Contains(c.Cmd, "--mysqld.address=db.internal:3306") && slices.Contains(c.Cmd, "--mysqld.username=test") && c.Env["MYSQLD_EXPORTER_PASSWORD"] == "example" {
					exporter = "mysql"
				}
			}
			// The runner measures the database through the workload's DSN.
			want := map[catalog.Protocol]string{catalog.ProtoPg: "postgres", catalog.ProtoMySQL: "mysql"}[tc.protocol]
			if exporter != want {
				t.Fatalf("exporter %q, want %q", exporter, want)
			}
		})
	}
}
