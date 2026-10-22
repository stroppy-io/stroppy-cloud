package provision

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBootScriptQuietsTheMachineThenRunsTheAgent(t *testing.T) {
	agent := "#!/bin/sh\nset -eu\necho 'install the agent'\n"
	out := bootScript(agent)
	if !strings.HasPrefix(out, "#!/bin/sh\n") {
		t.Fatalf("not a shell script:\n%s", out)
	}
	quiet := strings.Index(out, "systemctl mask unattended-upgrades")
	run := strings.Index(out, "exec /run/stroppy-agent-install")
	if quiet < 0 || run < 0 || quiet > run {
		t.Fatalf("upgrades are not stopped before the agent:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if b64, ok := strings.CutPrefix(line, "echo "); ok && strings.HasSuffix(b64, " | base64 -d > /run/stroppy-agent-install") {
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(b64, " | base64 -d > /run/stroppy-agent-install"))
			if err != nil || string(raw) != agent {
				t.Fatalf("agent script changed: %q %v", raw, err)
			}
			return
		}
	}
	t.Fatal("agent script not embedded")
}

func TestBootScriptOfNoAgentIsEmpty(t *testing.T) {
	if bootScript("") != "" {
		t.Fatal("a recording pass got a boot script")
	}
}
