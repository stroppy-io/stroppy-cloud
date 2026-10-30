// Package dbkind is what the product knows about each kind of database,
// short of deploying it: the catalog entry the forms show, the machine plan
// its parameters make, where a client connects and with which login, which
// protocols a workload may speak to it. One file per kind; the recipe that
// turns the plan into containers lives in compile, keyed by the same kind.
package dbkind

import (
	"fmt"
	"sort"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// Kind is one kind of database.
type Kind struct {
	catalog.Database
	// plan adds the kind's roles, flows, requirements, client endpoint and
	// label; the runner and its flow are added by Plan for every kind.
	plan func(p *topology.Plan, params map[string]any)
	// login is what a client signs in with (nil: no login).
	login func(params map[string]any) topology.Login
	// protocols a workload may speak (nil: Database.Protocols).
	protocols func(params map[string]any) []catalog.Protocol
	// unsupported: version → script → why that version cannot run it.
	unsupported map[string]map[string]unsupportedScript
	// unsupportedFor is the same by the database's parameters (a variant of
	// the kind rather than a version), nil when none.
	unsupportedFor func(params map[string]any) map[string]unsupportedScript
}

// unsupportedScript is a script an engine version cannot run.
type unsupportedScript struct {
	why string
}

// kinds in catalog order.
func kinds() []Kind {
	return []Kind{postgres(), orioledb(), mysql(), mariadb(), picodata(), ydb(), ydbManaged(), cockroach(), pgNoop(), noop(), external(), custom()}
}

// Databases is the catalog's database matrix.
func Databases() []catalog.Database {
	ks := kinds()
	out := make([]catalog.Database, len(ks))
	for i, k := range ks {
		out[i] = k.Database
	}
	return out
}

// Lookup returns one kind.
func Lookup(kind catalog.DatabaseKind) (Kind, bool) {
	for _, k := range kinds() {
		if k.Kind == kind {
			return k, true
		}
	}
	return Kind{}, false
}

// Plan builds the machine plan of a kind from its baked params. Params are
// the resolved db.<kind>.params@1 value (defaults applied), so every key
// the kind declares is present.
func Plan(kind catalog.DatabaseKind, params map[string]any) (topology.Plan, error) {
	k, ok := Lookup(kind)
	if !ok {
		return topology.Plan{}, fmt.Errorf("topology: unknown kind %q", kind)
	}
	p := topology.Plan{Kind: kind, Requirements: map[string]topology.Requirement{}}
	k.plan(&p, params)
	if k.login != nil {
		p.Client.Login = k.login(params)
	}
	// Every plan has a runner; its hardware comes from the workload.
	p.Add(topology.RoleRunner, "stroppy", 1)
	if p.Client.Role != "" {
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleRunner, To: p.Client.Role, Protocol: p.Client.Protocol, Port: p.Client.Port})
	}
	sort.SliceStable(p.Flows, func(i, j int) bool { return p.Flows[i].From+p.Flows[i].To < p.Flows[j].From+p.Flows[j].To })
	return p, nil
}

// Protocols are the protocols a workload may speak to a database of this
// kind with these params.
// UnsupportedScript says why a version of a kind cannot run a script (""
// when it can): a limit of that engine version, not of the protocol.
func UnsupportedScript(kind catalog.DatabaseKind, version string, params map[string]any, script string) string {
	k, ok := Lookup(kind)
	if !ok {
		return ""
	}
	u, ok := k.unsupported[version][script]
	if !ok && k.unsupportedFor != nil {
		u, ok = k.unsupportedFor(params)[script]
	}
	if !ok {
		return ""
	}
	return u.why
}

func Protocols(kind catalog.DatabaseKind, params map[string]any) []catalog.Protocol {
	k, ok := Lookup(kind)
	if !ok {
		return nil
	}
	if k.protocols != nil {
		return k.protocols(params)
	}
	return k.Protocols
}
