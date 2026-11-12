package compile

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"os"
)

func scriptParts(only string) mppScriptParts {
	return mppScriptParts{
		Ready: "/tmp/stroppy-mpp-ready", Self: "${ip:coordinator-1}", Hosts: "${ip:coordinator-1} ${ip:db-1} ${ip:db-2}", Segments: "${ip:db-1} ${ip:db-2}",
		CoordinatorDir: "/data/mpp/coordinator", PrimaryDir: "/data/mpp/primary", ScriptDir: "/etc/stroppy", CoordinatorOnly: only,
		Runner: "${ip:role:runner}", Port: 5432, User: "stroppy", Password: "stroppy", Database: "stroppy",
	}
}

// TestMPPSetGUC runs the script's set_guc against a fake gpconfig: a key that
// did not reach every instance's postgresql.conf stops the script, since
// gpconfig itself exits 0 then and the segment later fails to start.
func TestMPPSetGUC(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	_, clusterScript, err := mppScripts(scriptParts("--coordinatoronly"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(clusterScript, "set_guc() {")
	if start < 0 {
		t.Fatal("no set_guc in the cluster script")
	}
	end := strings.Index(clusterScript[start:], "\n}\n")
	if end < 0 {
		t.Fatal("set_guc does not end")
	}
	fn := clusterScript[start : start+end+3]
	for _, tc := range []struct {
		name, show string
		ok         bool
	}{
		{"consistent", "Values on all segments are consistent\nGUC : work_mem", true},
		{"a segment missed it", "WARNING: GUCS ARE OUT OF SYNC: \n[context: 0] [dbid: 2] [name: work_mem] [not set in file]", false},
		{"a host unreachable", "[ERROR]:-failed on host: db-4 msg:kex_exchange_identification: Connection closed by remote host", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bin := t.TempDir()
			fake := "#!/usr/bin/env bash\nif [ \"$1\" = -s ]; then printf '%s\\n' \"$SHOW\"; fi\nexit 0\n"
			if err := os.WriteFile(filepath.Join(bin, "gpconfig"), []byte(fake), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-c", "set -euo pipefail\n"+fn+"set_guc work_mem -v 819MB\necho applied")
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "SHOW="+tc.show)
			out, err := cmd.CombinedOutput()
			if got := err == nil && strings.Contains(string(out), "applied"); got != tc.ok {
				t.Errorf("set_guc ok = %v, want %v\n%s", got, tc.ok, out)
			}
		})
	}
}

func TestMPPScripts(t *testing.T) {
	t.Parallel()
	for _, only := range []string{"--masteronly", "--coordinatoronly"} {
		t.Run(only, func(t *testing.T) {
			t.Parallel()
			initScript, clusterScript, err := mppScripts(scriptParts(only))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(initScript+clusterScript, "{{") || strings.Contains(initScript+clusterScript, "<no value>") {
				t.Error("a template action is left in a script")
			}
			if !strings.Contains(clusterScript, "gpconfig -c \"$key\" -v \"${line#* = }\" "+only) {
				t.Errorf("the coordinator-only flag %s is not used", only)
			}
			// The scripts parse as bash: a quoting slip fails here, not on a stand.
			if _, err := exec.LookPath("bash"); err != nil {
				t.Skip("no bash")
			}
			dir := t.TempDir()
			for name, text := range map[string]string{"mpp-init.sh": initScript, "mpp-cluster.sh": clusterScript} {
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
					t.Errorf("%s does not parse: %v\n%s", name, err, out)
				}
			}
		})
	}
}

func BenchmarkMPPScripts(b *testing.B) {
	parts := scriptParts("--coordinatoronly")
	for b.Loop() {
		if _, _, err := mppScripts(parts); err != nil {
			b.Fatal(err)
		}
	}
}
