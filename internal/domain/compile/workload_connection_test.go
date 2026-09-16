package compile

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	schemasinfra "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
)

func TestConnectionOptionsReachDSNAndNativeDriver(t *testing.T) {
	for _, tc := range []struct {
		protocol catalog.Protocol
		dsn      string
		options  map[string]any
		want     string
	}{
		{catalog.ProtoPg, "postgres://u:p@${ip:role:db}:5432/db?sslmode=disable", map[string]any{"sslmode": "require", "application_name": "contract test"}, "application_name=contract+test&sslmode=require"},
		{catalog.ProtoCockroach, "postgres://u@${ip:role:db}:26257/db?sslmode=disable", map[string]any{"sslmode": "require"}, "sslmode=require"},
		{catalog.ProtoMySQL, "u:p?question@tcp(${ip:role:db}:3306)/db?parseTime=true", map[string]any{"tls": "preferred", "charset": "utf8mb4"}, "charset=utf8mb4&parseTime=true&tls=preferred"},
	} {
		got, err := connectionURL(tc.dsn, tc.protocol, tc.options)
		if err != nil || !strings.HasSuffix(got, "?"+tc.want) || (strings.Contains(tc.dsn, "p?question") && !strings.Contains(got, "p?question@")) {
			t.Fatalf("%s: %s %v", tc.protocol, got, err)
		}
	}
	registry := schemasinfra.New()
	cat, err := catalog.New(context.Background(), registry, registry)
	if err != nil {
		t.Fatal(err)
	}
	c := compilation{in: Input{Catalog: cat, Workload: library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: catalog.ProtoPg}, WorkloadBaked: json.RawMessage(`{"connection":{"kind":"pg","sslmode":"require","application_name":"contract","query_exec_mode":"exec"},"driver":{"postgres":{"statement_cache_capacity":0}},"segments":[]}`)}}
	if err := c.workload(); err != nil {
		t.Fatal(err)
	}
	driver := c.out.Spec.Workload.Driver
	for _, key := range []string{"sslmode", "applicationName", "tls", "charset"} {
		if _, ok := driver[key]; ok {
			t.Fatalf("URL option leaked into native driver: %s", key)
		}
	}
	pg := driver["postgres"].(map[string]any)
	if pg["statementCacheCapacity"] != float64(0) || pg["defaultQueryExecMode"] != "exec" {
		t.Fatalf("driver merge: %v", pg)
	}
}
