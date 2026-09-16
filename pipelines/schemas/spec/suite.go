package spec

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/provider"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/workload"
)

// Suite is spec.suite@1 — the params of the stroppy-suite pipeline: a list of
// cells, each carrying a whole RunSpec, plus how many run at once.
//
// Every cell is validated using the same Run schema as a standalone run.
//
// doc: STROPPY.MD §6.2, §16.7.
func Suite() *schemapb.Schema {
	return schemapb.NewSchema(ids.Spec("suite", 1)).
		Descr("SuiteSpec: the cells of a matrix run and how many of them run in parallel.").
		Strict().Coerce().
		DefSchema("run", Run()).
		DefSchema("segment", workload.Segment()).
		DefSchema("driver", workload.NativeDriver()).
		DefSchema("baseline", workload.Baseline()).
		DefSchema("yandex_settings", runProviderSettings(provider.YandexSettings())).
		DefSchema("aws_settings", runProviderSettings(provider.AwsSettings())).
		Fields(
			schemapb.Str("suite_run_id").Title("Suite run id").Group("Identity").
				Desc("Suite run id minted by the server; the parent Graphene run id.").
				Format(schemapb.FormatUUID).Required(),
			schemapb.Str("tenant").Title("Tenant").Group("Identity").
				Desc("Tenant slug; every child run lives in the same namespace.").
				Pattern(namePattern).Required(),

			schemapb.List("cells",
				schemapb.Object("",
					schemapb.Str("id").Title("Cell id").
						Desc("Stable cell id; the child run id is derived from parent + cell id.").
						Pattern(`^[a-z0-9][a-z0-9_-]{0,63}$`).Required(),
					schemapb.Ref("run_spec", "run").Title("RunSpec").
						Desc("A complete spec.run@1 value, validated before starting child runs.").
						Required(),
				).Strict(),
			).Title("Cells").Group("Cells").
				Desc("The resolved matrix: one child run per cell.").
				MinItems(1).MaxItems(256).Required(),

			schemapb.Int64("concurrency").Title("Concurrency").Group("Execution").
				Desc("How many cells run at the same time.").
				Gte(1).Lte(64).Default(1),

			schemapb.Object("defaults",
				schemapb.Bool("continue_on_failure").Title("Continue on failure").
					Desc("Keep running the remaining cells after one fails.").Default(true),
				schemapb.MapOf("labels", schemapb.Str("value").MaxLen(255)).
					Title("Labels").Desc("Labels stamped on every child run.").MaxEntries(32),
			).Title("Defaults").Group("Execution").
				Desc("Settings shared by every cell.").Strict(),
		).
		Rules(schemapb.Rule(
			`!("cells" in root) || root.cells.all(c, root.cells.filter(x, x.id == c.id).size() == 1)`,
			"cell ids must be unique",
		).ID("cell-ids-unique")).
		MustBuild()
}
