package dbkind_test

import (
	"slices"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
)

// The URL a workload gets for each kind, in each protocol the kind speaks:
// the login is the kind's own, whatever the protocol.
func TestClientURL(t *testing.T) {
	cases := []struct {
		kind   catalog.DatabaseKind
		params map[string]any
		proto  catalog.Protocol
		url    string
	}{
		{catalog.Postgres, nil, catalog.ProtoPg, "postgresql://postgres:stroppy_postgres@H:5432/postgres?sslmode=disable"},
		{catalog.Postgres, map[string]any{"haproxy": 1}, catalog.ProtoPg, "postgresql://postgres:stroppy_postgres@H:5000/postgres?sslmode=disable"},
		{catalog.PgNoop, nil, catalog.ProtoPg, "postgresql://postgres@H:5432/postgres?sslmode=disable"},
		{catalog.MySQL, nil, catalog.ProtoMySQL, "root:stroppy_mysql@tcp(H:3306)/stroppy?interpolateParams=true"},
		{catalog.Picodata, template(t, catalog.Picodata), catalog.ProtoPicodata, "postgres://admin:T0psecret@H:4327?sslmode=disable"},
		{catalog.YDB, nil, catalog.ProtoYDBGrpc, "grpc://H:2136/Root/stroppy"},
		{catalog.Cockroach, nil, catalog.ProtoCockroach, "postgresql://root@H:26257/defaultdb?sslmode=disable"},
		{catalog.Noop, nil, catalog.ProtoNoop, "noop://localhost"},
	}
	for _, tc := range cases {
		if tc.params == nil {
			tc.params = map[string]any{}
		}
		p, err := dbkind.Plan(tc.kind, bake(t, tc.kind, tc.params))
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := dbkind.URL(tc.proto, "H", p.Client, ""); got != tc.url {
			t.Errorf("%s/%s: %s, want %s", tc.kind, tc.proto, got, tc.url)
		}
	}
}

func TestExternalSpeaksItsDSNProtocol(t *testing.T) {
	got := dbkind.Protocols(catalog.External, map[string]any{"protocol": "mysql"})
	if len(got) != 1 || got[0] != catalog.ProtoMySQL {
		t.Fatalf("external protocols %v", got)
	}
}

// stroppy reaches Picodata only through its picodata driver over pgproto.
func TestPicodataSpeaksPicodataOverPgproto(t *testing.T) {
	got := dbkind.Protocols(catalog.Picodata, bake(t, catalog.Picodata, template(t, catalog.Picodata)))
	if len(got) != 1 || got[0] != catalog.ProtoPicodata {
		t.Fatalf("picodata protocols %v", got)
	}
	params := bake(t, catalog.Picodata, template(t, catalog.Picodata))
	params["pgproto"] = false
	if got := dbkind.Protocols(catalog.Picodata, params); got == nil || len(got) != 0 {
		t.Fatalf("picodata without pgproto speaks %v, want none", got)
	}
}

// template is the first catalog topology of a kind: params that bake.
func template(t *testing.T, kind catalog.DatabaseKind) map[string]any {
	t.Helper()
	k, ok := dbkind.Lookup(kind)
	if !ok || len(k.Topologies) == 0 {
		t.Fatalf("%s has no topology template", kind)
	}
	return k.Topologies[0].Params
}

// A self-hosted YDB stand has no TLS: only plain gRPC reaches it.
func TestSelfHostedYDBSpeaksPlainGRPC(t *testing.T) {
	for _, k := range dbkind.Databases() {
		if k.Kind != catalog.YDB {
			continue
		}
		if !slices.Equal(k.Protocols, []catalog.Protocol{catalog.ProtoYDBGrpc}) {
			t.Errorf("self-hosted YDB protocols = %v; want only ydb_grpc", k.Protocols)
		}
	}
}
