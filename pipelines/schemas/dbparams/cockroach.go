package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Cockroach is db.cockroach.params@1 — a CockroachDB cluster. Every node is
// the same: there is no primary, ranges are replicated by Raft, so the only
// real topology knobs are how many nodes there are and how they are told about
// their physical placement.
//
// doc: https://www.cockroachlabs.com/docs/stable/cockroach-start
func Cockroach() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("cockroach", 1)).
		Descr("CockroachDB topology and options: version, node count, locality, TLS mode.").
		Strict().Coerce().
		Fields(
			tuning(),
			// Supported lines on 2026-09-08. 23.2 LTS went EOL 2026-07-08 and
			// 24.2 (Innovation) died 2025-02-12, so both are gone; 24.1, 24.3,
			// 25.2 and 25.4 are the live LTS lines, 26.2 and 26.3 the current
			// Regular/Innovation ones.
			// doc: https://www.cockroachlabs.com/docs/releases/release-support-policy
			// doc: https://hub.docker.com/r/cockroachdb/cockroach/tags
			schemapb.Choice("version").Title("CockroachDB version").Group("Engine").
				Desc("Server series; the binary is pulled per release train (LTS lines get a year of maintenance plus a year of assistance).").
				Opt(schemapb.StrV("26.3"), "26.3 (Innovation)").
				Opt(schemapb.StrV("26.2"), "26.2 (Regular)").
				Opt(schemapb.StrV("25.4"), "25.4 LTS").
				Opt(schemapb.StrV("25.2"), "25.2 LTS").
				Opt(schemapb.StrV("24.3"), "24.3 LTS").
				Opt(schemapb.StrV("24.1"), "24.1 LTS").
				Default(schemapb.StrV("25.4")).Required(),

			// Raft needs a majority, so anything above one node must be odd:
			// an even cluster tolerates no more failures than the odd one
			// below it and costs a machine.
			// doc: https://www.cockroachlabs.com/docs/stable/cockroach-start
			schemapb.Int64("nodes").Title("Nodes").Group("Topology").
				Desc("Homogeneous cluster size; 1 for a single-node stand, otherwise an odd number >= 3 so Raft can hold a majority.").
				Gte(1).Lte(15).Default(3),

			// --locality is an ordered key=value list, most inclusive first,
			// and the key set must match across the whole cluster.
			// doc: https://www.cockroachlabs.com/docs/stable/cockroach-start
			schemapb.List("locality",
				schemapb.Str("").
					Desc("One node's --locality value: comma-separated key=value pairs, most inclusive first.").
					Pattern(`^[a-z][a-z0-9_\-]*=[A-Za-z0-9_.\-]+(,[a-z][a-z0-9_\-]*=[A-Za-z0-9_.\-]+)*$`).
					MaxLen(255),
			).Title("Per-node locality").Group("Topology").
				Desc("--locality for each node, in node order; leave empty for a flat cluster. When set it must have exactly one entry per node and every entry must use the same keys in the same order.").
				MaxItems(15),

			// doc: https://www.cockroachlabs.com/docs/stable/secure-a-cluster
			schemapb.Bool("insecure").Title("Insecure mode").Group("Security").
				Desc("Start with --insecure: no TLS and no authentication. Default on — a throwaway benchmark stand should measure the engine, not the handshake.").
				Default(true),

			// doc: https://www.haproxy.org/download/2.9/doc/configuration.txt
			schemapb.Int64("haproxy").Title("HAProxy nodes").Group("Routing").
				Desc("HAProxy instances spreading SQL clients over the nodes (cockroach gen haproxy produces an equivalent config).").
				Gte(0).Lte(2).Default(0),

			// doc: https://www.cockroachlabs.com/docs/stable/cockroach-sql
			schemapb.Str("init_sql").Title("Init SQL").Group("Initialization").
				Desc("SQL executed against the cluster once it is initialized, before the workload.").
				MaxLen(65536),
		).
		Rules(
			schemapb.Rule(`int(root.nodes) == 1 || (int(root.nodes) >= 3 && int(root.nodes) % 2 == 1)`,
				"a cluster is either a single node or an odd number of at least 3 (Raft majority)").ID("odd-nodes"),
			schemapb.Rule(`!("locality" in root) || size(root.locality) == 0 || size(root.locality) == int(root.nodes)`,
				"locality must be empty or carry exactly one entry per node").ID("locality-per-node"),
			schemapb.Rule(`int(root.haproxy) == 0 || int(root.nodes) >= 3`,
				"HAProxy in front of a single node only adds a hop").ID("haproxy-pointless").Warn(),
		).
		MustBuild()
}
