package agent

import (
	"strings"
	"testing"
)

func TestGenerateCloudInit_ContainsExpectedFields(t *testing.T) {
	params := CloudInitParams{
		BinaryURL:  "https://example.com/agent",
		ServerAddr: "http://server:8080",
		AgentPort:  9090,
		MachineID:  "vm-abc-123",
	}

	output, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("GenerateCloudInit failed: %v", err)
	}

	checks := []string{
		"#cloud-config",
		"STROPPY_SERVER_ADDR=http://server:8080",
		"STROPPY_AGENT_PORT=9090",
		"STROPPY_MACHINE_ID=vm-abc-123",
		"https://example.com/agent",
		RemoteBinPath,
		"stroppy-agent.service",
		"systemctl enable --now stroppy-agent",
		"curl -fsSL",
		"chmod +x",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("cloud-init output missing expected content: %q", check)
		}
	}
}

func TestGenerateCloudInit_DefaultPort(t *testing.T) {
	params := CloudInitParams{
		BinaryURL:  "https://example.com/agent",
		ServerAddr: "http://server:8080",
		MachineID:  "vm-1",
		// AgentPort omitted — should default to DefaultAgentPort (9090)
	}

	output, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("GenerateCloudInit failed: %v", err)
	}

	expected := "STROPPY_AGENT_PORT=9090"
	if !strings.Contains(output, expected) {
		t.Errorf("expected default port in output, missing %q", expected)
	}
}

func TestGenerateCloudInit_ExtraEnv(t *testing.T) {
	params := CloudInitParams{
		BinaryURL:  "https://example.com/agent",
		ServerAddr: "http://server:8080",
		AgentPort:  9090,
		MachineID:  "vm-env",
		ExtraEnv: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	output, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("GenerateCloudInit failed: %v", err)
	}

	if !strings.Contains(output, "CUSTOM_VAR=custom_value") {
		t.Error("cloud-init output missing extra env CUSTOM_VAR=custom_value")
	}
}

func TestGenerateCloudInit_StartsWithCloudConfig(t *testing.T) {
	params := CloudInitParams{
		BinaryURL:  "https://example.com/agent",
		ServerAddr: "http://server:8080",
		MachineID:  "vm-2",
	}

	output, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("GenerateCloudInit failed: %v", err)
	}

	if !strings.HasPrefix(output, "#cloud-config") {
		t.Error("cloud-init output should start with #cloud-config")
	}
}

func TestGenerateCloudInit_ContainsUserSetup(t *testing.T) {
	params := CloudInitParams{
		BinaryURL:  "https://example.com/agent",
		ServerAddr: "http://server:8080",
		MachineID:  "vm-3",
	}

	output, err := GenerateCloudInit(params)
	if err != nil {
		t.Fatalf("GenerateCloudInit failed: %v", err)
	}

	if !strings.Contains(output, "name: stroppy") {
		t.Error("cloud-init should create stroppy user")
	}
	if !strings.Contains(output, "NOPASSWD:ALL") {
		t.Error("cloud-init should grant sudo access")
	}
}
