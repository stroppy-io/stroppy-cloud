package catalog

import (
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/workload"
)

// The schemas own parameter descriptions and defaults. Catalog only adds build
// identities and protocol availability, never a second handwritten parameter list.
func stroppy() StroppyCatalog {
	all := []Protocol{ProtoPg, ProtoMySQL, ProtoPicodata, ProtoYDBGrpc, ProtoYDBGrpcs, ProtoCockroach, ProtoNoop}
	procs := []Protocol{ProtoPg, ProtoMySQL, ProtoCockroach, ProtoNoop}
	// doc: stroppy workloads/tpch and workloads/tpcds ship pg, mysql, pico and
	// ydb dialects only; CockroachDB has its own SQL for tpcc and tpcb alone.
	noCockroach := []Protocol{ProtoPg, ProtoMySQL, ProtoPicodata, ProtoYDBGrpc, ProtoYDBGrpcs, ProtoNoop}
	scripts := []StroppyScript{
		{ID: "tpcc/tx", Title: "TPC-C, raw transactions", Protocols: all},
		{ID: "tpcc/procs", Title: "TPC-C, stored procedures", Protocols: procs},
		{ID: "tpcb/tx", Title: "TPC-B, raw transactions", Protocols: all},
		{ID: "tpcb/procs", Title: "TPC-B, stored procedures", Protocols: procs},
		{ID: "tpch/tx", Title: "TPC-H", Protocols: noCockroach},
		{ID: "tpcds", Title: "TPC-DS", Protocols: noCockroach, Description: "Picodata: load only. YDB: baked queries only. MySQL generated streams omit queries 51, 88 and 97."},
		{ID: "simple", Title: "Simple key-value", Protocols: all},
		{ID: "execute_sql", Title: "Execute SQL", Protocols: all},
		{ID: "baseline", Title: "Baseline workload", Protocols: all, Description: "A baseline workload segment; the machine self-check remains separately configurable."},
	}
	schema := workload.Segment()
	var variants map[string]*schemapb.Schema
	var runFields []*schemapb.Schema_Field
	for _, f := range schema.GetFields() {
		if f.GetName() == "workload" {
			variants = f.GetOneOf().GetVariants()
		}
		if f.GetName() == "run" {
			runFields = f.GetObject().GetSchema().GetFields()
		}
	}
	for i := range scripts {
		sc := &scripts[i]
		for _, name := range workload.Steps(sc.ID) {
			phase := "bootstrap"
			if name == "workload" {
				phase = "workload"
			}
			sc.Steps = append(sc.Steps, StroppyStep{ID: name, Phase: phase})
		}
		for _, f := range variants[sc.ID].GetFields() {
			if f.GetName() != "script" {
				sc.Params = append(sc.Params, schemaParam(f, "workload"))
			}
		}
		for _, f := range runFields {
			sc.Params = append(sc.Params, schemaParam(f, "run"))
		}
	}
	// 6.2 rejects the crdb driver for tpcb/procs (stroppy-io/stroppy#229).
	scripts62 := make([]StroppyScript, len(scripts))
	copy(scripts62, scripts)
	for i := range scripts62 {
		if scripts62[i].ID == "tpcb/procs" {
			scripts62[i].Protocols = []Protocol{ProtoPg, ProtoMySQL, ProtoNoop}
		}
	}
	return StroppyCatalog{Source: "static", Versions: []StroppyVersion{
		{Version: "6.0.0", Image: "ghcr.io/stroppy-io/stroppy:v6.0.0.62", Baseline: true, Protocols: all, Scripts: scripts},
		{Version: "6.1.0", Image: "ghcr.io/stroppy-io/stroppy:v6.1.0.63", Baseline: true, Protocols: all, Scripts: scripts},
		// 6.2 names drivers ("default", see stroppycfg.DriverKey) and writes
		// report envelope schema 4; workload parameters are a superset of 6.1.
		{Version: "6.2.1", Image: "ghcr.io/stroppy-io/stroppy:v6.2.1.67", Default: true, Baseline: true, Protocols: all, Scripts: scripts62},
	}}
}

func schemaParam(f *schemapb.Schema_Field, scope string) StroppyParam {
	parts := strings.Split(f.GetName(), "_")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	p := StroppyParam{Name: strings.ReplaceAll(f.GetName(), "_", "-"), Config: strings.Join(parts, ""), Scope: scope, Description: f.GetDescription(), Env: strings.ToUpper(f.GetName())}
	switch {
	case f.GetInt64() != nil:
		p.Type = "int64"
		if v := f.GetInt64().Default; v != nil {
			p.Default = *v
		}
	case f.GetDouble() != nil:
		p.Type = "float64"
		if v := f.GetDouble().Default; v != nil {
			p.Default = *v
		}
	case f.GetBool() != nil:
		p.Type = "bool"
		if v := f.GetBool().Default; v != nil {
			p.Default = *v
		}
	case f.GetDuration() != nil:
		p.Type = "duration"
		if v := f.GetDuration().GetDefault(); v != nil {
			p.Default = v.AsDuration().String()
		}
	case f.GetChoice() != nil:
		p.Type = "string"
		if v := f.GetChoice().GetDefault(); v != nil {
			p.Default = v.GetStringValue()
		}
	default:
		p.Type = "string"
		if v := f.GetString_().Default; v != nil {
			p.Default = *v
		}
	}
	if f.GetNullable() {
		p.DefaultDescription = f.GetDescription()
	}
	return p
}
