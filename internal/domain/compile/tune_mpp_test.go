package compile

import "testing"

func TestMPPMemoryOf(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                      string
		ramMB, instances, queries int
		want                      mppMemory
	}{
		// SynxDB's host: 128 GB, 8 primaries — their 15000 MB is the same 90%.
		{"128 GB × 8, one query", 131072, 8, 1, mppMemory{VmemMB: 14745, StatementMB: 11058, MaxStatementMB: 13270, WorkMB: 1024, CacheMB: 8192}},
		{"128 GB × 16, one query", 131072, 16, 1, mppMemory{VmemMB: 7372, StatementMB: 5529, MaxStatementMB: 6634, WorkMB: 819, CacheMB: 4096}},
		{"128 GB × 16, five queries", 131072, 16, 5, mppMemory{VmemMB: 7372, StatementMB: 1105, MaxStatementMB: 6634, WorkMB: 819, CacheMB: 4096}},
		{"16 GB × 2, one query", 16384, 2, 1, mppMemory{VmemMB: 7372, StatementMB: 5529, MaxStatementMB: 6634, WorkMB: 819, CacheMB: 4096}},
		{"8 GB coordinator", 8192, 1, 1, mppMemory{VmemMB: 7372, StatementMB: 5529, MaxStatementMB: 6634, WorkMB: 819, CacheMB: 4096}},
		// Many queries cannot push statement_mem under the engine's own 125 MB.
		{"floor of statement_mem", 16384, 2, 128, mppMemory{VmemMB: 7372, StatementMB: 125, MaxStatementMB: 6634, WorkMB: 819, CacheMB: 4096}},
		// work_mem has the engine's 32 MB as its floor and 1 GB as its ceiling.
		{"work_mem floor", 4096, 16, 1, mppMemory{VmemMB: 230, StatementMB: 172, MaxStatementMB: 207, WorkMB: 32, CacheMB: 128}},
		{"zero queries count as one", 16384, 2, 0, mppMemory{VmemMB: 7372, StatementMB: 5529, MaxStatementMB: 6634, WorkMB: 819, CacheMB: 4096}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mppMemoryOf(tc.ramMB, tc.instances, tc.queries)
			if got != tc.want {
				t.Errorf("mppMemoryOf(%d, %d, %d) = %+v, want %+v", tc.ramMB, tc.instances, tc.queries, got, tc.want)
			}
			if got.StatementMB >= got.MaxStatementMB {
				t.Errorf("statement_mem %d is not below max_statement_mem %d", got.StatementMB, got.MaxStatementMB)
			}
			if got.MaxStatementMB > got.VmemMB {
				t.Errorf("max_statement_mem %d is above gp_vmem_protect_limit %d", got.MaxStatementMB, got.VmemMB)
			}
		})
	}
}

func TestMPPOvercommitRatio(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		ramMB, total int
		want         int
	}{
		{"90% of 128 GB", 131072, 117964, 97},
		{"the safe formula's half", 131072, 67600, 98},
		{"all of it", 131072, 131072, 97},
		{"nothing", 131072, 0, 100},
		{"never below 50", 1024, 1000000, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := mppOvercommitRatio(tc.ramMB, tc.total); got != tc.want {
				t.Errorf("mppOvercommitRatio(%d, %d) = %d, want %d", tc.ramMB, tc.total, got, tc.want)
			}
		})
	}
}

func TestMPPDirty(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                      string
		ramMB                     int
		wantBackground, wantDirty int
	}{
		{"8 GB", 8192, 3, 10},
		{"64 GB", 65536, 3, 10},
		{"128 GB", 131072, 1, 3},
		{"256 GB", 262144, 1, 2},
		{"1 TB", 1048576, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			background, dirty := mppDirty(tc.ramMB)
			if background != tc.wantBackground || dirty != tc.wantDirty {
				t.Errorf("mppDirty(%d) = %d, %d, want %d, %d", tc.ramMB, background, dirty, tc.wantBackground, tc.wantDirty)
			}
			if background > dirty {
				t.Errorf("background ratio %d above dirty ratio %d", background, dirty)
			}
		})
	}
}

func BenchmarkMPPMemoryOf(b *testing.B) {
	for b.Loop() {
		mppMemoryOf(131072, 16, 5)
	}
}
