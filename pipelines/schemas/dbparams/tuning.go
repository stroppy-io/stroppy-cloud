package dbparams

import schemapb "github.com/gopherex/schemapb/go/schemapb"

// tuning is the common tuning mode of a deployed engine. auto sizes the
// engine's caches, logs, I/O budget, connection limits and the machine's
// kernel from the machine it lands on, its role, the topology and the
// workload (docs/tuning/README.md); stock renders the upstream defaults.
// A key set in a role's config always wins over auto.
func tuning() *schemapb.ChoiceB {
	return schemapb.Choice("tuning").Title("Tuning").Group("Tuning").
		Desc("auto = settings sized from the machine, role, topology and workload (every value the run used is in its configs); stock = the engine's own defaults. A key set in the config wins over auto.").
		Opt(schemapb.StrV("auto"), "Auto").
		Opt(schemapb.StrV("stock"), "Engine defaults").
		Default(schemapb.StrV("auto"))
}
