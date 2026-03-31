package types

import (
	"testing"
)

func TestDefaultPackages_PostgresVersions(t *testing.T) {
	pkgs := DefaultPackages()

	expectedVersions := []string{"16", "17"}
	for _, v := range expectedVersions {
		ps, ok := pkgs.Postgres[v]
		if !ok {
			t.Errorf("missing postgres version %s", v)
			continue
		}
		if len(ps.Apt) == 0 {
			t.Errorf("postgres %s: no apt packages", v)
		}
		if len(ps.Rpm) == 0 {
			t.Errorf("postgres %s: no rpm packages", v)
		}
		if len(ps.PreInstallApt) == 0 {
			t.Errorf("postgres %s: no pre-install apt commands", v)
		}
		if len(ps.PreInstallRpm) == 0 {
			t.Errorf("postgres %s: no pre-install rpm commands", v)
		}
	}
}

func TestDefaultPackages_MySQLVersions(t *testing.T) {
	pkgs := DefaultPackages()

	expectedVersions := []string{"8.0", "8.4"}
	for _, v := range expectedVersions {
		ps, ok := pkgs.MySQL[v]
		if !ok {
			t.Errorf("missing mysql version %s", v)
			continue
		}
		if len(ps.Apt) == 0 {
			t.Errorf("mysql %s: no apt packages", v)
		}
		if len(ps.Rpm) == 0 {
			t.Errorf("mysql %s: no rpm packages", v)
		}
	}
}

func TestDefaultPackages_PicodataVersions(t *testing.T) {
	pkgs := DefaultPackages()

	expectedVersions := []string{"25.3"}
	for _, v := range expectedVersions {
		ps, ok := pkgs.Picodata[v]
		if !ok {
			t.Errorf("missing picodata version %s", v)
			continue
		}
		if len(ps.Apt) == 0 {
			t.Errorf("picodata %s: no apt packages", v)
		}
		if len(ps.Rpm) == 0 {
			t.Errorf("picodata %s: no rpm packages", v)
		}
		if len(ps.PreInstallApt) == 0 {
			t.Errorf("picodata %s: no pre-install apt commands", v)
		}
		if len(ps.PreInstallRpm) == 0 {
			t.Errorf("picodata %s: no pre-install rpm commands", v)
		}
	}
}

func TestDefaultPackages_PostgresAptContainsExpectedPackage(t *testing.T) {
	pkgs := DefaultPackages()
	ps := pkgs.Postgres["16"]

	found := false
	for _, p := range ps.Apt {
		if p == "postgresql-16" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("postgres 16 apt packages should contain postgresql-16, got %v", ps.Apt)
	}
}

func TestDefaultPackages_MonitoringAndStroppy(t *testing.T) {
	pkgs := DefaultPackages()

	// Monitoring and Stroppy are installed from binaries, not packages.
	// Their PackageSet should be empty but present.
	if pkgs.Monitoring.Apt != nil && len(pkgs.Monitoring.Apt) > 0 {
		t.Errorf("monitoring should have empty apt packages, got %v", pkgs.Monitoring.Apt)
	}
	if pkgs.Stroppy.Apt != nil && len(pkgs.Stroppy.Apt) > 0 {
		t.Errorf("stroppy should have empty apt packages, got %v", pkgs.Stroppy.Apt)
	}
}

func TestPackageSet_Fields(t *testing.T) {
	ps := PackageSet{
		Apt:           []string{"pkg1", "pkg2"},
		Rpm:           []string{"pkg3"},
		PreInstallApt: []string{"cmd1"},
		PreInstallRpm: []string{"cmd2"},
		CustomRepoApt: "deb https://repo apt main",
		CustomRepoKey: "https://repo/key.gpg",
		CustomRepoRpm: "https://repo/rpm",
		DebFiles:      []string{"https://example.com/a.deb"},
		RpmFiles:      []string{"https://example.com/b.rpm"},
	}

	if len(ps.Apt) != 2 {
		t.Errorf("expected 2 apt packages, got %d", len(ps.Apt))
	}
	if len(ps.Rpm) != 1 {
		t.Errorf("expected 1 rpm package, got %d", len(ps.Rpm))
	}
	if ps.CustomRepoApt == "" {
		t.Error("CustomRepoApt should not be empty")
	}
	if len(ps.DebFiles) != 1 {
		t.Errorf("expected 1 deb file, got %d", len(ps.DebFiles))
	}
	if len(ps.RpmFiles) != 1 {
		t.Errorf("expected 1 rpm file, got %d", len(ps.RpmFiles))
	}
}
