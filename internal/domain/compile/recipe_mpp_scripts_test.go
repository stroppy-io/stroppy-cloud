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
