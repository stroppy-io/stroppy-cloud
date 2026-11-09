package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// The Greenplum family: five engines that share one deployment shape (a
// coordinator, segment hosts, gpinitsystem over ssh) and one parameter form.
// The mpp-builds images carry the same runtime contract for all of them.
//
// doc: https://github.com/stroppy-io/mpp-builds — "Image contract"

// mppVersion is one selectable version of an MPP engine.
type mppVersion struct {
	version, label string
}

// Greenplum is db.greenplum.params@1.
func Greenplum() *schemapb.Schema {
	return mppParams("greenplum", "Greenplum", "7.1.0", []mppVersion{
		{"7.1.0", "7.1.0 (PostgreSQL 12)"},
		{"6.27.1", "6.27.1 (PostgreSQL 9.4)"},
	})
}

// Greengage is db.greengage.params@1. 8.0.0-alpha.1 is a pre-release.
func Greengage() *schemapb.Schema {
	return mppParams("greengage", "Greengage", "7.5.1", []mppVersion{
		{"8.0.0-alpha.1", "8.0.0-alpha.1 (pre-release, PostgreSQL 14)"},
		{"7.5.1", "7.5.1 (PostgreSQL 12)"},
		{"6.32.0", "6.32.0 (PostgreSQL 9.4)"},
	})
}

// Warehousepg is db.warehousepg.params@1.
func Warehousepg() *schemapb.Schema {
	return mppParams("warehousepg", "WarehousePG", "7.6.1", []mppVersion{
		{"7.6.1", "7.6.1 (PostgreSQL 12)"},
		{"6.27.7", "6.27.7 (PostgreSQL 9.4)"},
	})
}

// Opengpdb is db.opengpdb.params@1.
func Opengpdb() *schemapb.Schema {
	return mppParams("opengpdb", "OpenGPDB", "6.30.2", []mppVersion{
		{"6.30.2", "6.30.2 (PostgreSQL 9.4)"},
	})
}

// Cloudberry is db.cloudberry.params@1.
func Cloudberry() *schemapb.Schema {
	return mppParams("cloudberry", "Apache Cloudberry", "2.1.0", []mppVersion{
		{"2.1.0", "2.1.0 incubating (PostgreSQL 14)"},
	})
}

func mppParams(kind, title, def string, versions []mppVersion) *schemapb.Schema {
	version := schemapb.Choice("version").Title(title + " version").Group("Engine").
		Desc("Server release; the image is pulled per release. The PostgreSQL base of the release decides the spelling of the coordinator in gpinitsystem (MASTER_* up to 9.4, COORDINATOR_* from 12).")
	for _, v := range versions {
		version = version.Opt(schemapb.StrV(v.version), v.label)
	}
	return schemapb.NewSchema(ids.DB(kind, 1)).
		Descr(title+" cluster: one coordinator and a number of segment hosts. No mirrors and no standby coordinator are offered: a failed segment stops the cluster, which is what a benchmark wants to see.").
		Strict().Coerce().
		Fields(
			tuning(),
			version.Default(schemapb.StrV(def)).Required(),

			// doc: https://techdocs.broadcom.com/us/en/vmware-tanzu/data-solutions/tanzu-greenplum/7/greenplum-database/install_guide-init_gpdb.html — hostfile of segment hosts
			schemapb.Int64("segment_hosts").Title("Segment hosts").Group("Topology").
				Desc("Machines that carry the primary segments; the coordinator is one more machine.").
				Gte(1).Lte(32).Default(4),

			// doc: https://techdocs.broadcom.com/us/en/vmware-tanzu/data-solutions/tanzu-greenplum/7/greenplum-database/best_practices-sysconfig.html — "usually a value between 4 and 8"; Arenadata ADB defaults to the number of cores / 2
			schemapb.Int64("segments_per_host").Title("Segments per host").Group("Topology").
				Desc("Primary segments on each segment host. 0 = auto: half of the host's vCPUs, at least 1 (what Arenadata ADB defaults to).").
				Gte(0).Lte(64).Default(0),

			// doc: https://techdocs.broadcom.com/us/en/vmware-tanzu/data-solutions/tanzu-greenplum/7/greenplum-database/ref_guide-config_params-guc-list.html#optimizer
			schemapb.Choice("optimizer").Title("Query optimizer").Group("Engine").
				Desc("Which planner the cluster uses: GPORCA (the optimizer GUC on) or the PostgreSQL-based planner (off). Set cluster-wide; every release ships with GPORCA built in.").
				Opt(schemapb.StrV("orca"), "GPORCA").
				Opt(schemapb.StrV("postgres"), "PostgreSQL planner").
				Default(schemapb.StrV("orca")),
		).
		MustBuild()
}
