package types

// PackageManager is the system package manager type.
type PackageManager string

const (
	PackageManagerApt PackageManager = "apt"
	PackageManagerRpm PackageManager = "rpm"
)

// PackageSet defines packages to install for a component.
type PackageSet struct {
	Apt []string `json:"apt,omitempty"` // debian/ubuntu packages
	Rpm []string `json:"rpm,omitempty"` // rhel/centos packages
	// PreInstall are shell commands to run before package install (add repos, keys, etc.)
	PreInstallApt []string `json:"pre_install_apt,omitempty"`
	PreInstallRpm []string `json:"pre_install_rpm,omitempty"`
}

// PackageDefaults holds default package sets for all components.
type PackageDefaults struct {
	Postgres   map[string]PackageSet `json:"postgres"` // version -> packages
	MySQL      map[string]PackageSet `json:"mysql"`    // version -> packages
	Picodata   map[string]PackageSet `json:"picodata"` // version -> packages
	Monitoring PackageSet            `json:"monitoring"`
	Stroppy    PackageSet            `json:"stroppy"`
}

// DefaultPackages returns PackageDefaults with real-world defaults for all
// supported databases and components.
func DefaultPackages() PackageDefaults {
	return PackageDefaults{
		Postgres: map[string]PackageSet{
			"16": {
				Apt: []string{"postgresql-16", "postgresql-client-16"},
				PreInstallApt: []string{
					`sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
					"wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -",
					"apt-get update",
				},
				Rpm: []string{"postgresql16-server", "postgresql16"},
				PreInstallRpm: []string{
					"dnf install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %rhel)-x86_64/pgdg-redhat-repo-latest.noarch.rpm",
				},
			},
			"17": {
				Apt: []string{"postgresql-17", "postgresql-client-17"},
				PreInstallApt: []string{
					`sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
					"wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -",
					"apt-get update",
				},
				Rpm: []string{"postgresql17-server", "postgresql17"},
				PreInstallRpm: []string{
					"dnf install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %rhel)-x86_64/pgdg-redhat-repo-latest.noarch.rpm",
				},
			},
		},
		MySQL: map[string]PackageSet{
			"8.0": {
				Apt: []string{"mysql-server-8.0", "mysql-client"},
				Rpm: []string{"mysql-community-server", "mysql-community-client"},
			},
			"8.4": {
				Apt: []string{"mysql-server-8.4", "mysql-client"},
				Rpm: []string{"mysql-community-server", "mysql-community-client"},
			},
		},
		Picodata: map[string]PackageSet{
			"25.3": {
				Apt: []string{"picodata"},
				PreInstallApt: []string{
					`curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg`,
					`echo "deb https://download.picodata.io/tarantool-picodata/ubuntu/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/picodata.list`,
					"apt-get update",
				},
				Rpm: []string{"picodata"},
				PreInstallRpm: []string{
					`sh -c 'cat > /etc/yum.repos.d/picodata.repo << REPO
[picodata]
name=Picodata
baseurl=https://binary.picodata.io/repository/picodata-rpm/el$releasever
gpgcheck=0
enabled=1
REPO'`,
				},
			},
		},
		// Monitoring is installed from binary tarballs, not packages
		// (matching the metrics_runner.sh approach).
		Monitoring: PackageSet{},
		// Stroppy is installed from GitHub releases binary.
		Stroppy: PackageSet{},
	}
}
