package compile_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

func TestExternalDSNPreservedForEveryProtocol(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	cases := []struct {
		protocol    catalog.Protocol
		dsn, driver string
	}{
		{catalog.ProtoPg, "postgres://test:example@db.internal:5432/bench", "postgres"},
		{catalog.ProtoMySQL, "test:example@tcp(db.internal:3306)/bench", "mysql"},
		{catalog.ProtoPicodata, "postgres://admin:example@db.internal:4327", "picodata"},
		{catalog.ProtoCockroach, "postgres://test:example@db.internal:26257/bench", "postgres"},
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
			workload, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: tc.protocol, Segments: []json.RawMessage{json.RawMessage(`{"name":"main","workload":{"script":"simple"},"run":{"vus":2,"duration":"30s"}}`)}})
			if err != nil {
				t.Fatal(err)
			}
			out, err := compile.Compile(ctx, reg, compile.Input{RunID: uuid.New(), Tenant: "test", Database: database, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs, Workload: workload, WorkloadBaked: baked, Sizes: map[string]library.RoleSize{"runner": {Size: "S"}}, Provider: provider, ProviderKind: "yandex", ProviderSettings: json.RawMessage(`{"cloud_id":"b1g","folder_id":"b1g","zone":"ru-central1-a"}`), CredentialsSecret: "yc", Catalog: cat})
			if err != nil {
				t.Fatal(err)
			}
			if out.Spec.Workload.URL != tc.dsn || out.Spec.Workload.DriverType != tc.driver {
				t.Fatal("external DSN or driver replaced with generated endpoint")
			}
			if len(out.Spec.Machines) != 1 || out.Spec.Machines[0].Role != "runner" {
				t.Fatal("external test must only provision its runner")
			}
			for _, c := range out.Spec.Containers {
				if c.Role != "runner" {
					t.Fatal("external database component declared")
				}
			}
		})
	}
}
