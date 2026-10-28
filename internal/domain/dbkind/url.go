package dbkind

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// URL is the stroppy driver URL and driver type of an endpoint spoken to in
// a protocol. Host is an address or a ${ip:...} placeholder; sslmode applies
// to the PostgreSQL protocol ("" = disable).
func URL(proto catalog.Protocol, host string, ep topology.Endpoint, sslmode string) (url, driver string) {
	if ep.URL != "" {
		return ep.URL, Driver(proto)
	}
	l := ep.Login
	switch proto { //nolint:exhaustive // pg is the default branch
	case catalog.ProtoNoop:
		return "noop://localhost", Driver(proto)
	case catalog.ProtoMySQL:
		// Without interpolateParams go-sql-driver prepares, executes and
		// closes a statement per query: three round trips where pgx, with
		// its statement cache, makes one.
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?interpolateParams=true", l.User, l.Password, host, ep.Port, l.Database), Driver(proto)
	case catalog.ProtoPicodata:
		return fmt.Sprintf("postgres://%s@%s:%d?sslmode=disable", userinfo(l), host, ep.Port), Driver(proto)
	case catalog.ProtoYDBGrpc:
		return fmt.Sprintf("grpc://%s:%d%s", host, ep.Port, l.Database), Driver(proto)
	case catalog.ProtoYDBGrpcs:
		return fmt.Sprintf("grpcs://%s:%d%s", host, ep.Port, l.Database), Driver(proto)
	case catalog.ProtoCockroach:
		return fmt.Sprintf("postgresql://%s@%s:%d/%s?sslmode=disable", userinfo(l), host, ep.Port, l.Database), Driver(proto)
	default: // catalog.ProtoPg
		if sslmode == "" {
			sslmode = "disable"
		}
		return fmt.Sprintf("postgresql://%s@%s:%d/%s?sslmode=%s", userinfo(l), host, ep.Port, l.Database, sslmode), Driver(proto)
	}
}

// Driver is stroppy's driverType for a protocol.
func Driver(p catalog.Protocol) string {
	switch p {
	case catalog.ProtoMySQL:
		return "mysql"
	case catalog.ProtoPicodata:
		return "picodata"
	case catalog.ProtoYDBGrpc, catalog.ProtoYDBGrpcs:
		return "ydb"
	case catalog.ProtoNoop:
		return "noop"
	case catalog.ProtoPg, catalog.ProtoCockroach:
		return "postgres"
	default:
		return "postgres"
	}
}

func userinfo(l topology.Login) string {
	if l.Password == "" {
		return l.User
	}
	return l.User + ":" + l.Password
}
