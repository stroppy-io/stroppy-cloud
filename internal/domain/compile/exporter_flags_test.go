package compile

import (
	"slices"
	"testing"
)

func TestNodeExporterCmd(t *testing.T) {
	t.Parallel()
	cmd := nodeExporterCmd()
	for _, want := range []string{"--path.rootfs=/host", "--collector.cpu.info", "--collector.processes", "--collector.cgroups"} {
		if !slices.Contains(cmd, want) {
			t.Errorf("missing %s in %v", want, cmd)
		}
	}
	// On a stand these cannot work: the docker-default AppArmor profile denies
	// D-Bus (systemd, logind) and /proc/slabinfo is root-only; on, they log an
	// error every scrape.
	for _, off := range []string{"--collector.systemd", "--collector.logind", "--collector.slabinfo"} {
		if slices.Contains(cmd, off) {
			t.Errorf("%s is on", off)
		}
	}
}

func TestPostgresExporterCmd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		oriole bool
		want   []string
		absent []string
	}{
		{"postgres", false, []string{"--collector.stat_statements", "--collector.postmaster"}, []string{"--no-collector.stat_user_tables"}},
		{"orioledb", true, []string{"--no-collector.stat_user_tables", "--extend.query-path=" + orioledbQueriesPath}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := postgresExporterCmd(tc.oriole)
			for _, w := range tc.want {
				if !slices.Contains(cmd, w) {
					t.Errorf("missing %s in %v", w, cmd)
				}
			}
			for _, a := range tc.absent {
				if slices.Contains(cmd, a) {
					t.Errorf("%s present", a)
				}
			}
		})
	}
}

func TestMysqldExporterCmd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		gr   bool
		want bool
	}{
		{"single", false, false},
		{"group replication", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := mysqldExporterCmd("db-1:3306", "exporter", tc.gr)
			if !slices.Contains(cmd, "--mysqld.address=db-1:3306") || !slices.Contains(cmd, "--collect.global_status") {
				t.Errorf("base flags missing: %v", cmd)
			}
			if got := slices.Contains(cmd, "--collect.perf_schema.replication_group_members"); got != tc.want {
				t.Errorf("group replication tables = %v, want %v", got, tc.want)
			}
		})
	}
}

func BenchmarkNodeExporterCmd(b *testing.B) {
	for b.Loop() {
		nodeExporterCmd()
	}
}
