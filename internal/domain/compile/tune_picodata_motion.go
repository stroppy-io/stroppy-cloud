package compile

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// --- Picodata: the virtual table of an analytic query ---------------------------
//
// docs/tuning/picodata.md §2.5. A query that joins or aggregates across
// shards collects the rows of a motion in a virtual table on the instance
// that took the query (the router) and sends it to the storages as a
// temporary space; sql_motion_row_max stops the collection and so is the only
// guard between a query and the memory of the machine. Measured on 26.2.1
// (8 instances, TPC-H SF 0.1 and 1, cold start, anonymous memory): the need of
// a query is the row count of the relation it moves — lineitem for q3, q5,
// q7-q10, q12, q18, q21 — or groups × instances for an aggregate (q1: 4
// groups, 32 rows on 8 instances); the router holds 0.3-4.2 KB per row
// (4 columns to a join of 6 tables), every storage instance up to 0.8 KB, and
// both are linear in rows from SF 0.1 to SF 1.

const (
	// picoTPCHLineitemRows is the row count of lineitem at SF 1 (TPC-H spec
	// §4.2.5; stroppy loads 6001215), the largest relation of the suite.
	picoTPCHLineitemRows = 6001215
	// picoMotionHeadroom covers the queries whose motion is a little larger
	// than a table: q20 moved 1.08 × the rows of partsupp.
	picoMotionHeadroom = 1.1
	// picoRouterRowBytes is the most the router held per motion row (q5 on
	// SF 0.1: 2399 MB for 600572 rows = 4188 B); picoStorageRowBytes the most
	// one storage instance held (q5: 455 MB = 794 B). Every instance of a
	// machine keeps its own copy of the table, so a machine pays
	// router + instances × storage per row.
	picoRouterRowBytes  = 4200
	picoStorageRowBytes = 800
	// picoDataBytesPerSF is what the TPC-H data takes in memtx per unit of
	// scale factor (SF 1: 8 instances at 305 MB less 62 MB of idle instance).
	picoDataBytesPerSF = 2 << 30
	// picoInstanceBaseMB is what an idle instance holds outside the arena.
	picoInstanceBaseMB = 62
)

// picoShape is where the database runs: the instances, how they are dealt
// over machines, and one machine's memory.
type picoShape struct {
	instances, perMachine, machines, memoryMB int
}

// picoAnalytic is what the workload asks of the motions: the rows of the
// largest relation of the analytic segments at their scale factor and the
// queries that run at once, and whether any other segment is present.
// Only TPC-H is sized: the pico script of TPC-DS loads data but its queries
// do not parse (comma joins), so it moves no rows.
type picoAnalytic struct {
	scale  float64
	vus    int
	others bool
}

// picoAnalyticOf reads the segments of the workload.
func (c *compilation) picoAnalyticOf() picoAnalytic {
	var a picoAnalytic
	for _, raw := range c.in.Workload.Segments {
		var s spec.Segment
		if json.Unmarshal(raw, &s) != nil {
			a.others = true
			continue
		}
		if !strings.HasPrefix(s.Workload.Script, "tpch/") {
			a.others = true
			continue
		}
		sf := numberOf(s.Workload.Params["scale_factor"])
		if sf == 0 {
			sf = 1
		}
		a.scale = max(a.scale, sf)
		a.vus = max(a.vus, int(s.Run.VUs))
	}
	a.vus = max(a.vus, 1)
	return a
}

// fixedBytes is the memory of a machine left after the OS and the idle
// instances.
func (s picoShape) fixedBytes() float64 {
	return float64(s.memoryMB-picoOSReserveMB(s.memoryMB)-s.perMachine*picoInstanceBaseMB) * (1 << 20)
}

// dataBytes is the share of the data of scale one machine holds.
func (s picoShape) dataBytes(scale float64) float64 {
	return scale * picoDataBytesPerSF / float64(max(1, s.machines))
}

// rowBytes is what one motion row costs a machine.
func (s picoShape) rowBytes() float64 {
	return picoRouterRowBytes + float64(s.perMachine)*picoStorageRowBytes
}

// motionBudget is how many motion rows one of vus concurrent queries may
// collect before the machine's memory is gone.
func (s picoShape) motionBudget(scale float64, vus int) float64 {
	return (s.fixedBytes() - s.dataBytes(scale)) / float64(vus) / s.rowBytes()
}

// maxScale is the largest scale factor whose heaviest motion fits:
//
//	(fixed − SF × data) / vus / rowBytes ≥ headroom × lineitem × SF
func (s picoShape) maxScale(vus int) float64 {
	perSF := s.dataBytes(1) + float64(vus)*s.rowBytes()*picoMotionHeadroom*picoTPCHLineitemRows
	return max(0, s.fixedBytes()/perSF)
}

// picoMotionRows is the auto sql_motion_row_max. Without an analytic segment
// it is the per-instance rule of the OLTP workloads (picoMotionRowAuto × the
// instances: every instance sends its partial result of an aggregate to the
// router). An analytic segment needs the rows of its largest relation; when
// that does not fit the machine's memory for its concurrent queries the run
// is refused, because a smaller limit would fail most of the suite and a
// larger one would exhaust the memory.
func (c *compilation) picoMotionRows(s picoShape) (int, error) {
	oltp := picoMotionRowAuto * max(1, s.instances)
	a := c.picoAnalyticOf()
	if a.scale == 0 {
		return oltp, nil
	}
	need := int(math.Ceil(picoMotionHeadroom * picoTPCHLineitemRows * a.scale))
	if s.memoryMB > 0 {
		budget := s.motionBudget(a.scale, a.vus)
		if float64(need) > budget {
			refusal := errs.Newf(errs.CodeInvalid, "TPC-H scale factor %g needs %d motion rows per query, the machine holds %d", a.scale, need, int(max(0, budget)))
			return 0, refusal.Because("picodata_scale_exceeds_memory",
				"scale_factor", strconv.FormatFloat(a.scale, 'f', -1, 64),
				"max_scale_factor", strconv.FormatFloat(math.Floor(s.maxScale(a.vus)*100)/100, 'f', -1, 64),
				"memory_gb", strconv.Itoa(s.memoryMB>>10),
				"instances_per_machine", strconv.Itoa(s.perMachine),
				"concurrent_queries", strconv.Itoa(a.vus))
		}
	}
	if a.others {
		return max(oltp, need), nil
	}
	return need, nil
}
