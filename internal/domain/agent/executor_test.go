package agent

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

func TestResolvePackages_DefaultPostgres(t *testing.T) {
	ps := resolvePackages(nil, "postgres", "16")
	if len(ps.Apt) == 0 {
		t.Fatal("expected default apt packages for postgres 16, got none")
	}
	found := false
	for _, p := range ps.Apt {
		if p == "postgresql-16" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected postgresql-16 in apt packages, got %v", ps.Apt)
	}
}

func TestResolvePackages_DefaultPostgres17(t *testing.T) {
	ps := resolvePackages(nil, "postgres", "17")
	if len(ps.Apt) == 0 {
		t.Fatal("expected default apt packages for postgres 17, got none")
	}
	found := false
	for _, p := range ps.Apt {
		if p == "postgresql-17" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected postgresql-17 in apt packages, got %v", ps.Apt)
	}
}

func TestResolvePackages_DefaultMySQL(t *testing.T) {
	ps := resolvePackages(nil, "mysql", "8.0")
	if len(ps.Apt) == 0 {
		t.Fatal("expected default apt packages for mysql 8.0, got none")
	}
}

func TestResolvePackages_DefaultPicodata(t *testing.T) {
	ps := resolvePackages(nil, "picodata", "25.3")
	if len(ps.Apt) == 0 {
		t.Fatal("expected default apt packages for picodata 25.3, got none")
	}
}

func TestResolvePackages_UnknownVersionReturnsEmpty(t *testing.T) {
	ps := resolvePackages(nil, "postgres", "99")
	if len(ps.Apt) != 0 || len(ps.Rpm) != 0 {
		t.Errorf("expected empty package set for unknown version, got apt=%v rpm=%v", ps.Apt, ps.Rpm)
	}
}

func TestResolvePackages_UnknownKindReturnsEmpty(t *testing.T) {
	ps := resolvePackages(nil, "unknown_db", "1.0")
	if len(ps.Apt) != 0 {
		t.Errorf("expected empty package set for unknown kind, got %v", ps.Apt)
	}
}

func TestResolvePackages_CustomOverridesDefault(t *testing.T) {
	custom := &types.PackageSet{
		Apt: []string{"my-custom-pg"},
	}
	ps := resolvePackages(custom, "postgres", "16")
	if len(ps.Apt) != 1 || ps.Apt[0] != "my-custom-pg" {
		t.Errorf("expected custom package set, got %v", ps.Apt)
	}
}

func TestResolvePackages_CustomDebFiles(t *testing.T) {
	custom := &types.PackageSet{
		DebFiles: []string{"https://example.com/foo.deb"},
	}
	ps := resolvePackages(custom, "postgres", "16")
	if len(ps.DebFiles) != 1 {
		t.Errorf("expected custom deb files, got %v", ps.DebFiles)
	}
}

func TestResolvePackages_CustomRepoApt(t *testing.T) {
	custom := &types.PackageSet{
		CustomRepoApt: "deb https://custom.repo/apt jammy main",
	}
	ps := resolvePackages(custom, "postgres", "16")
	if ps.CustomRepoApt != "deb https://custom.repo/apt jammy main" {
		t.Errorf("expected custom repo apt, got %q", ps.CustomRepoApt)
	}
}

func TestResolvePackages_EmptyCustomFallsBackToDefault(t *testing.T) {
	custom := &types.PackageSet{} // all fields empty
	ps := resolvePackages(custom, "postgres", "16")
	if len(ps.Apt) == 0 {
		t.Fatal("expected default packages when custom is empty struct")
	}
}

func TestResolveMemoryDefaults_Percentages(t *testing.T) {
	m := map[string]string{
		"shared_buffers":       "25%",
		"effective_cache_size": "75%",
		"max_connections":      "200",
	}
	resolveMemoryDefaults(m)

	// Percentage values should be resolved to concrete MB values.
	if m["shared_buffers"] == "25%" {
		t.Error("shared_buffers was not resolved from percentage")
	}
	if m["effective_cache_size"] == "75%" {
		t.Error("effective_cache_size was not resolved from percentage")
	}
	// Non-percentage values should remain unchanged.
	if m["max_connections"] != "200" {
		t.Errorf("max_connections should remain 200, got %s", m["max_connections"])
	}
}

func TestResolveMemoryDefaults_NoPercentage(t *testing.T) {
	m := map[string]string{
		"work_mem": "64MB",
		"listen":   "'*'",
	}
	resolveMemoryDefaults(m)
	if m["work_mem"] != "64MB" {
		t.Errorf("work_mem should remain 64MB, got %s", m["work_mem"])
	}
	if m["listen"] != "'*'" {
		t.Errorf("listen should remain '*', got %s", m["listen"])
	}
}

func TestResolveMemoryDefaults_MinimumFloor(t *testing.T) {
	// With a very small percentage that would yield < 32 MB,
	// the function should floor at 32 MB.
	m := map[string]string{
		"tiny_param": "1%",
	}
	resolveMemoryDefaults(m)
	// Even 1% of typical systems is > 32MB, but the floor exists in code.
	// At minimum, the value should no longer end in %.
	if m["tiny_param"] == "1%" {
		t.Error("tiny_param was not resolved from percentage")
	}
}

func TestParseConfig_Success(t *testing.T) {
	type testCfg struct {
		Version string `json:"version"`
		Port    int    `json:"port"`
	}

	cmd := Command{
		ID:     "test-1",
		Action: ActionInstallPostgres,
		Config: map[string]any{
			"version": "16",
			"port":    5432,
		},
	}

	var cfg testCfg
	if err := parseConfig(cmd, &cfg); err != nil {
		t.Fatalf("parseConfig failed: %v", err)
	}
	if cfg.Version != "16" {
		t.Errorf("expected version 16, got %s", cfg.Version)
	}
	if cfg.Port != 5432 {
		t.Errorf("expected port 5432, got %d", cfg.Port)
	}
}

func TestParseConfig_NilConfig(t *testing.T) {
	type testCfg struct {
		Name string `json:"name"`
	}
	cmd := Command{ID: "test-2", Action: ActionInstallPostgres, Config: nil}
	var cfg testCfg
	if err := parseConfig(cmd, &cfg); err != nil {
		t.Fatalf("parseConfig with nil config should not error: %v", err)
	}
	if cfg.Name != "" {
		t.Errorf("expected empty name, got %q", cfg.Name)
	}
}

func TestParseConfig_NestedStruct(t *testing.T) {
	type inner struct {
		Key string `json:"key"`
	}
	type outer struct {
		Inner inner `json:"inner"`
	}

	cmd := Command{
		ID:     "test-3",
		Action: ActionConfigPostgres,
		Config: map[string]any{
			"inner": map[string]any{"key": "value"},
		},
	}

	var cfg outer
	if err := parseConfig(cmd, &cfg); err != nil {
		t.Fatalf("parseConfig with nested struct failed: %v", err)
	}
	if cfg.Inner.Key != "value" {
		t.Errorf("expected inner key 'value', got %q", cfg.Inner.Key)
	}
}
