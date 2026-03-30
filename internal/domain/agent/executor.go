package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// Executor runs agent commands on the local machine.
type Executor struct {
	aptMu        sync.Mutex // only for apt-get operations
	bootstrapMu  sync.Once
	bootstrapErr error
}

// NewExecutor returns a new Executor.
func NewExecutor() *Executor { return &Executor{} }

// bootstrap installs base utilities required by all actions. Runs once, thread-safe.
func (e *Executor) bootstrap(ctx context.Context) error {
	e.bootstrapMu.Do(func() {
		// Prevent services from auto-starting during apt install (Docker has no systemd).
		_, _ = shell(ctx, `printf '#!/bin/sh\nexit 101\n' > /usr/sbin/policy-rc.d && chmod +x /usr/sbin/policy-rc.d`)
		_, err := e.shellWithAptLock(ctx, "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "+
			"curl wget ca-certificates gnupg lsb-release sudo tar gzip python3-pip")
		if err != nil {
			e.bootstrapErr = fmt.Errorf("bootstrap: %w", err)
		}
	})
	return e.bootstrapErr
}

// shellWithAptLock runs a shell command while holding the apt mutex.
func (e *Executor) shellWithAptLock(ctx context.Context, script string) (string, error) {
	e.aptMu.Lock()
	defer e.aptMu.Unlock()
	return shell(ctx, script)
}

// aptInstall runs apt-get install with the apt lock held to prevent concurrent apt operations.
func (e *Executor) aptInstall(ctx context.Context, packages string) error {
	_, err := e.shellWithAptLock(ctx, "DEBIAN_FRONTEND=noninteractive apt-get install -y "+packages)
	return err
}

// aptPreInstall runs pre-install commands (repo setup) with the apt lock held.
func (e *Executor) aptPreInstall(ctx context.Context, cmds []string) error {
	for _, cmd := range cmds {
		if _, err := e.shellWithAptLock(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

// Run executes a Command and returns a Report.
func (e *Executor) Run(ctx context.Context, cmd Command) Report {
	report := Report{CommandID: cmd.ID, Status: ReportCompleted}

	if err := e.bootstrap(ctx); err != nil {
		report.Status = ReportFailed
		report.Error = err.Error()
		return report
	}

	var err error
	switch cmd.Action {
	case ActionInstallPostgres:
		err = e.installPostgres(ctx, cmd)
	case ActionConfigPostgres:
		err = e.configPostgres(ctx, cmd)
	case ActionInstallMySQL:
		err = e.installMySQL(ctx, cmd)
	case ActionConfigMySQL:
		err = e.configMySQL(ctx, cmd)
	case ActionInstallPicodata:
		err = e.installPicodata(ctx, cmd)
	case ActionConfigPicodata:
		err = e.configPicodata(ctx, cmd)
	case ActionInstallMonitor:
		err = e.installMonitor(ctx, cmd)
	case ActionConfigMonitor:
		err = e.configMonitor(ctx, cmd)
	case ActionInstallStroppy:
		err = e.installStroppy(ctx, cmd)
	case ActionRunStroppy:
		err = e.runStroppy(ctx, cmd)
	case ActionInstallEtcd:
		err = e.installEtcd(ctx, cmd)
	case ActionConfigEtcd:
		err = e.configEtcd(ctx, cmd)
	case ActionInstallPatroni:
		err = e.installPatroni(ctx, cmd)
	case ActionConfigPatroni:
		err = e.configPatroni(ctx, cmd)
	case ActionInstallPgBouncer:
		err = e.installPgBouncer(ctx, cmd)
	case ActionConfigPgBouncer:
		err = e.configPgBouncer(ctx, cmd)
	case ActionInstallHAProxy:
		err = e.installHAProxy(ctx, cmd)
	case ActionConfigHAProxy:
		err = e.configHAProxy(ctx, cmd)
	case ActionInstallProxySQL:
		err = e.installProxySQL(ctx, cmd)
	case ActionConfigProxySQL:
		err = e.configProxySQL(ctx, cmd)
	default:
		err = fmt.Errorf("unknown action: %s", cmd.Action)
	}

	if err != nil {
		report.Status = ReportFailed
		report.Error = err.Error()
	}
	return report
}

// shell runs a bash script capturing combined stdout+stderr output.
func shell(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", script)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	output := buf.String()
	if err != nil {
		return output, fmt.Errorf("%w: %s", err, output)
	}
	return output, nil
}

// resolveMemoryDefaults replaces percentage placeholders (e.g. "25%") with
// concrete memory values computed from the system's total RAM.
func resolveMemoryDefaults(m map[string]string) {
	totalMB := getTotalMemoryMB()
	for k, v := range m {
		if strings.HasSuffix(v, "%") {
			pctStr := strings.TrimSuffix(v, "%")
			var pct int
			fmt.Sscanf(pctStr, "%d", &pct)
			if pct > 0 {
				valMB := totalMB * pct / 100
				if valMB < 32 {
					valMB = 32
				}
				// Cap at 2GB for Docker containers.
				if valMB > 2048 {
					valMB = 2048
				}
				m[k] = fmt.Sprintf("%dMB", valMB)
			}
		}
	}
}

// getTotalMemoryMB reads total system memory from /proc/meminfo.
// Falls back to 1024 MB if unable to read.
func getTotalMemoryMB() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 1024
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			var kb int
			fmt.Sscanf(line, "MemTotal: %d kB", &kb)
			if kb > 0 {
				return kb / 1024
			}
		}
	}
	return 1024
}

// parseConfig marshals cmd.Config through JSON into the target struct.
func parseConfig(cmd Command, target any) error {
	data, err := json.Marshal(cmd.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal config into %T: %w", target, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// installPostgres
// ---------------------------------------------------------------------------

func (e *Executor) installPostgres(ctx context.Context, cmd Command) error {
	var cfg PostgresInstallConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	version := cfg.Version
	if version == "" {
		version = "16"
	}

	pkgs := types.DefaultPackages()
	ps, ok := pkgs.Postgres[version]
	if !ok {
		return fmt.Errorf("unsupported postgres version: %s", version)
	}

	if err := e.aptPreInstall(ctx, ps.PreInstallApt); err != nil {
		return fmt.Errorf("pre-install: %w", err)
	}
	if err := e.aptInstall(ctx, strings.Join(ps.Apt, " ")); err != nil {
		return fmt.Errorf("install postgres packages: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// configPostgres
// ---------------------------------------------------------------------------

func (e *Executor) configPostgres(ctx context.Context, cmd Command) error {
	var cfg PostgresClusterConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	version := cfg.Version
	if version == "" {
		version = "16"
	}

	defaults := types.PostgresDefaults(version)
	resolveMemoryDefaults(defaults)
	for k, v := range cfg.Options {
		defaults[k] = v
	}

	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n")
	for k, v := range defaults {
		fmt.Fprintf(&confBuf, "%s = %s\n", k, v)
	}

	confDir := fmt.Sprintf("/etc/postgresql/%s/main", version)
	confPath := confDir + "/postgresql.conf"
	writeScript := fmt.Sprintf("cat >> %s << 'PGCONF'\n\n# stroppy-agent overrides\n%sPGCONF", confPath, confBuf.String())
	if _, err := shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write postgresql.conf: %w", err)
	}

	hbaPath := confDir + "/pg_hba.conf"
	hbaContent := `# Generated by stroppy-agent
local   all             all                 trust
host    all             all   127.0.0.1/32  trust
host    all             all   ::1/128       trust
host    all             all   0.0.0.0/0     trust
host    replication     all   0.0.0.0/0     trust
local   replication     all                 trust
`
	hbaScript := fmt.Sprintf("cat > %s << 'PGHBA'\n%sPGHBA", hbaPath, hbaContent)
	if _, err := shell(ctx, hbaScript); err != nil {
		return fmt.Errorf("write pg_hba.conf: %w", err)
	}

	if cfg.Role == "master" {
		startScript := fmt.Sprintf(`pg_ctlcluster %s main start || true
# Wait for postgres to be ready.
for i in $(seq 1 30); do
  pg_isready -U postgres && break
  sleep 1
done`, version)
		if _, err := shell(ctx, startScript); err != nil {
			return fmt.Errorf("start postgres: %w", err)
		}
	} else {
		// Replica: set up streaming replication from master.
		masterHost := cfg.MasterHost
		if masterHost == "" {
			return fmt.Errorf("replica requires master_host")
		}

		dataDir := fmt.Sprintf("/var/lib/postgresql/%s/main", version)
		replicaScript := fmt.Sprintf(`pg_ctlcluster %s main stop || true
rm -rf %s/*
sudo -u postgres pg_basebackup -h %s -D %s -U postgres -Fp -Xs -P -R
chown -R postgres:postgres %s
pg_ctlcluster %s main start`, version, dataDir, masterHost, dataDir, dataDir, version)

		if _, err := shell(ctx, replicaScript); err != nil {
			return fmt.Errorf("setup replica: %w", err)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// installMySQL
// ---------------------------------------------------------------------------

func (e *Executor) installMySQL(ctx context.Context, cmd Command) error {
	var cfg MySQLInstallConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	version := cfg.Version
	if version == "" {
		version = "8.0"
	}

	pkgs := types.DefaultPackages()
	ps, ok := pkgs.MySQL[version]
	if !ok {
		return fmt.Errorf("unsupported mysql version: %s", version)
	}

	if err := e.aptPreInstall(ctx, ps.PreInstallApt); err != nil {
		return fmt.Errorf("pre-install: %w", err)
	}
	// MySQL postinst tries to start/stop mysqld which fails in Docker.
	// Install with error tolerance, then verify binary exists.
	e.aptInstall(ctx, strings.Join(ps.Apt, " "))
	if _, err := shell(ctx, "which mysqld"); err != nil {
		return fmt.Errorf("mysql binary not found after install")
	}
	return nil
}

// ---------------------------------------------------------------------------
// configMySQL
// ---------------------------------------------------------------------------

func (e *Executor) configMySQL(ctx context.Context, cmd Command) error {
	var cfg MySQLClusterConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	version := "8.0"
	defaults := types.MySQLDefaults(version)
	resolveMemoryDefaults(defaults)
	for k, v := range cfg.Options {
		defaults[k] = v
	}

	// Build my.cnf content. MySQL uses M/G suffixes (not MB/GB like PG).
	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n[mysqld]\n")
	for k, v := range defaults {
		v = strings.ReplaceAll(v, "MB", "M")
		v = strings.ReplaceAll(v, "GB", "G")
		fmt.Fprintf(&confBuf, "%s = %s\n", k, v)
	}

	confPath := "/etc/mysql/my.cnf"
	writeScript := fmt.Sprintf("mkdir -p /etc/mysql && cat > %s << 'MYCNF'\n%sMYCNF", confPath, confBuf.String())
	if _, err := shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write my.cnf: %w", err)
	}

	dataDir := "/var/lib/mysql"
	// Kill any leftover mysqld from postinst, init fresh if needed, start.
	initScript := fmt.Sprintf(`mkdir -p /var/log/mysql /var/run/mysqld && chown mysql:mysql /var/log/mysql /var/run/mysqld 2>/dev/null
pkill -9 mysqld 2>/dev/null; sleep 1
if [ ! -f %s/ibdata1 ]; then
  rm -rf %s/*
  mysqld --initialize-insecure --datadir=%s --user=mysql
fi`, dataDir, dataDir, dataDir)
	if _, err := shell(ctx, initScript); err != nil {
		return fmt.Errorf("init mysql: %w", err)
	}

	startScript := fmt.Sprintf(`nohup mysqld --defaults-file=/etc/mysql/my.cnf --datadir=%s --user=mysql > /var/log/mysql/mysqld.log 2>&1 &
for i in $(seq 1 30); do mysqladmin ping -u root --silent 2>/dev/null && break; sleep 1; done`, dataDir)
	if _, err := shell(ctx, startScript); err != nil {
		return fmt.Errorf("start mysql: %w", err)
	}

	if cfg.Role == "replica" && cfg.PrimaryHost != "" {
		// Wait extra for mysqld to be fully ready on replica.
		shell(ctx, `for i in $(seq 1 30); do mysqladmin ping -h 127.0.0.1 -u root --silent 2>/dev/null && break; sleep 1; done`)
		replicaScript := fmt.Sprintf(`mysql -h 127.0.0.1 -u root -e "CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_USER='repl', SOURCE_AUTO_POSITION=1; START REPLICA;"`,
			cfg.PrimaryHost)
		if _, err := shell(ctx, replicaScript); err != nil {
			return fmt.Errorf("setup mysql replica: %w", err)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// installPicodata
// ---------------------------------------------------------------------------

func (e *Executor) installPicodata(ctx context.Context, cmd Command) error {
	var cfg PicodataInstallConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	version := cfg.Version
	if version == "" {
		version = "25.3"
	}

	pkgs := types.DefaultPackages()
	ps, ok := pkgs.Picodata[version]
	if !ok {
		return fmt.Errorf("unsupported picodata version: %s", version)
	}

	if err := e.aptPreInstall(ctx, ps.PreInstallApt); err != nil {
		return fmt.Errorf("pre-install: %w", err)
	}
	if err := e.aptInstall(ctx, strings.Join(ps.Apt, " ")); err != nil {
		return fmt.Errorf("install picodata packages: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// configPicodata
// ---------------------------------------------------------------------------

func (e *Executor) configPicodata(ctx context.Context, cmd Command) error {
	var cfg PicodataClusterConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	defaults := types.PicodataDefaults("25.3")
	for k, v := range cfg.Options {
		defaults[k] = v
	}

	// Build picodata config.
	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n")
	fmt.Fprintf(&confBuf, "instance_id = %d\n", cfg.InstanceID)
	fmt.Fprintf(&confBuf, "replication_factor = %d\n", cfg.Replication)
	fmt.Fprintf(&confBuf, "shards = %d\n", cfg.Shards)
	for k, v := range defaults {
		// Skip keys already written explicitly.
		if k == "replication_factor" || k == "shards" {
			continue
		}
		fmt.Fprintf(&confBuf, "%s = %s\n", k, v)
	}

	confPath := "/etc/picodata/picodata.conf"
	writeScript := fmt.Sprintf("mkdir -p /etc/picodata && cat > %s << 'PICOCONF'\n%sPICOCONF", confPath, confBuf.String())
	if _, err := shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write picodata config: %w", err)
	}

	// Start picodata with peer list.
	peerAddrs := strings.Join(cfg.Peers, ",")
	dataDir := "/var/lib/picodata"
	startScript := fmt.Sprintf("mkdir -p %s && picodata run --config %s --peers %s --data-dir %s &",
		dataDir, confPath, peerAddrs, dataDir)
	if _, err := shell(ctx, startScript); err != nil {
		return fmt.Errorf("start picodata: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// installMonitor
// ---------------------------------------------------------------------------

func (e *Executor) installMonitor(ctx context.Context, cmd Command) error {
	settings := types.DefaultServerSettings()
	neVer := settings.Monitoring.NodeExporterVersion
	peVer := settings.Monitoring.PostgresExporterVersion

	// Download and install node_exporter.
	neURL := fmt.Sprintf(
		"https://github.com/prometheus/node_exporter/releases/download/v%s/node_exporter-%s.linux-amd64.tar.gz",
		neVer, neVer,
	)
	neScript := fmt.Sprintf(
		`curl -fsSL "%s" -o /tmp/node_exporter.tar.gz && `+
			`tar xzf /tmp/node_exporter.tar.gz -C /tmp && `+
			`cp /tmp/node_exporter-%s.linux-amd64/node_exporter /usr/local/bin/node_exporter && `+
			`chmod +x /usr/local/bin/node_exporter && `+
			`rm -rf /tmp/node_exporter*`,
		neURL, neVer,
	)
	if _, err := shell(ctx, neScript); err != nil {
		return fmt.Errorf("install node_exporter: %w", err)
	}

	// Download and install postgres_exporter.
	peURL := fmt.Sprintf(
		"https://github.com/prometheus-community/postgres_exporter/releases/download/v%s/postgres_exporter-%s.linux-amd64.tar.gz",
		peVer, peVer,
	)
	peScript := fmt.Sprintf(
		`curl -fsSL "%s" -o /tmp/postgres_exporter.tar.gz && `+
			`tar xzf /tmp/postgres_exporter.tar.gz -C /tmp && `+
			`cp /tmp/postgres_exporter-%s.linux-amd64/postgres_exporter /usr/local/bin/postgres_exporter && `+
			`chmod +x /usr/local/bin/postgres_exporter && `+
			`rm -rf /tmp/postgres_exporter*`,
		peURL, peVer,
	)
	if _, err := shell(ctx, peScript); err != nil {
		return fmt.Errorf("install postgres_exporter: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// configMonitor
// ---------------------------------------------------------------------------

func (e *Executor) configMonitor(ctx context.Context, cmd Command) error {
	var cfg MonitorSetupConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	// Write a simple prometheus scrape config (used by vmagent or standalone prom).
	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\nglobal:\n  scrape_interval: 15s\nscrape_configs:\n")
	confBuf.WriteString("  - job_name: node\n    static_configs:\n      - targets:\n")
	for _, t := range cfg.ScrapeTargets {
		fmt.Fprintf(&confBuf, "          - '%s'\n", t)
	}

	confPath := "/etc/prometheus/prometheus.yml"
	writeScript := fmt.Sprintf("mkdir -p /etc/prometheus && cat > %s << 'PROMCFG'\n%sPROMCFG", confPath, confBuf.String())
	if _, err := shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write prometheus config: %w", err)
	}

	// Start node_exporter as a background process (no systemd in docker).
	if _, err := shell(ctx, "nohup /usr/local/bin/node_exporter > /var/log/node_exporter.log 2>&1 &"); err != nil {
		return fmt.Errorf("start node_exporter: %w", err)
	}

	// Start postgres_exporter as a background process.
	peScript := `export DATA_SOURCE_NAME="postgresql://postgres@localhost:5432/postgres?sslmode=disable"
nohup /usr/local/bin/postgres_exporter > /var/log/postgres_exporter.log 2>&1 &`
	if _, err := shell(ctx, peScript); err != nil {
		return fmt.Errorf("start postgres_exporter: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// installStroppy
// ---------------------------------------------------------------------------

func (e *Executor) installStroppy(ctx context.Context, cmd Command) error {
	var cfg StroppyInstallConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	version := cfg.Version
	if version == "" {
		version = "3.1.0"
	}

	dlURL := fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/v%s/stroppy_linux_amd64.tar.gz", version)

	script := fmt.Sprintf(
		`curl -fsSL "%s" -o /tmp/stroppy.tar.gz && `+
			`tar xzf /tmp/stroppy.tar.gz -C /tmp && `+
			`cp /tmp/stroppy /usr/local/bin/stroppy && `+
			`chmod +x /usr/local/bin/stroppy && `+
			`rm -rf /tmp/stroppy*`,
		dlURL,
	)
	if _, err := shell(ctx, script); err != nil {
		return fmt.Errorf("install stroppy: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// runStroppy
// ---------------------------------------------------------------------------

func (e *Executor) runStroppy(ctx context.Context, cmd Command) error {
	var cfg StroppyRunConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	// Build driver URL based on db kind.
	var driverURL string
	switch cfg.DBKind {
	case "postgres":
		driverURL = fmt.Sprintf("postgresql://postgres@%s:%d/postgres?sslmode=disable", cfg.DBHost, cfg.DBPort)
	case "mysql":
		driverURL = fmt.Sprintf("root@tcp(%s:%d)/", cfg.DBHost, cfg.DBPort)
	case "picodata":
		driverURL = fmt.Sprintf("postgresql://admin@%s:%d/admin?sslmode=disable", cfg.DBHost, cfg.DBPort)
	default:
		driverURL = fmt.Sprintf("%s:%d", cfg.DBHost, cfg.DBPort)
	}

	// Build env vars. Stroppy v4 uses STROPPY_DRIVER_0 (JSON format) for driver config.
	driverJSON := fmt.Sprintf(`{"url":"%s","driverType":"%s"}`, driverURL, cfg.DBKind)
	var envParts []string
	envParts = append(envParts, fmt.Sprintf("STROPPY_DRIVER_0='%s'", driverJSON))
	envParts = append(envParts, fmt.Sprintf("DRIVER_URL='%s'", driverURL))
	for k, v := range cfg.Options {
		if strings.HasPrefix(k, "K6_OTEL_") || strings.HasPrefix(k, "k6_otel_") {
			envParts = append(envParts, fmt.Sprintf("%s='%s'", strings.ToUpper(k), v))
		}
	}

	duration := cfg.Duration
	if duration == "" {
		duration = "60s"
	}

	workers := cfg.Workers
	if workers == 0 {
		workers = 10
	}

	var exports strings.Builder
	for _, e := range envParts {
		fmt.Fprintf(&exports, "export %s\n", e)
	}
	script := fmt.Sprintf("%sstroppy run %s -- --duration %s --vus %d",
		exports.String(), cfg.Workload, duration, workers)

	if _, err := shell(ctx, script); err != nil {
		return fmt.Errorf("run stroppy: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// installEtcd
// ---------------------------------------------------------------------------

func (e *Executor) installEtcd(ctx context.Context, cmd Command) error {
	var cfg EtcdInstallConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	version := cfg.Version
	if version == "" {
		version = "3.5.17"
	}

	// Download etcd from GitHub releases.
	dlURL := fmt.Sprintf(
		"https://github.com/etcd-io/etcd/releases/download/v%s/etcd-v%s-linux-amd64.tar.gz",
		version, version,
	)
	script := fmt.Sprintf(
		`curl -fsSL "%s" -o /tmp/etcd.tar.gz && `+
			`tar xzf /tmp/etcd.tar.gz -C /tmp && `+
			`cp /tmp/etcd-v%s-linux-amd64/etcd /usr/local/bin/etcd && `+
			`cp /tmp/etcd-v%s-linux-amd64/etcdctl /usr/local/bin/etcdctl && `+
			`chmod +x /usr/local/bin/etcd /usr/local/bin/etcdctl && `+
			`rm -rf /tmp/etcd*`,
		dlURL, version, version,
	)
	if _, err := shell(ctx, script); err != nil {
		return fmt.Errorf("install etcd: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// configEtcd
// ---------------------------------------------------------------------------

func (e *Executor) configEtcd(ctx context.Context, cmd Command) error {
	var cfg EtcdClusterConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	if cfg.State == "" {
		cfg.State = "new"
	}
	if cfg.ClientURL == "" {
		cfg.ClientURL = "http://0.0.0.0:2379"
	}
	if cfg.PeerURL == "" {
		cfg.PeerURL = "http://0.0.0.0:2380"
	}

	// Write environment file for etcd.
	envContent := fmt.Sprintf(`ETCD_NAME=%s
ETCD_INITIAL_CLUSTER=%s
ETCD_INITIAL_CLUSTER_STATE=%s
ETCD_LISTEN_CLIENT_URLS=%s
ETCD_LISTEN_PEER_URLS=%s
ETCD_ADVERTISE_CLIENT_URLS=%s
ETCD_INITIAL_ADVERTISE_PEER_URLS=%s
`, cfg.Name, cfg.InitialCluster, cfg.State,
		cfg.ClientURL, cfg.PeerURL,
		cfg.AdvertiseClient, cfg.AdvertisePeer)

	writeScript := fmt.Sprintf("cat > /etc/default/etcd << 'ETCDENV'\n%sETCDENV", envContent)
	if _, err := shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write etcd env: %w", err)
	}

	// Create data directory and start etcd.
	dataDir := "/var/lib/etcd"
	startScript := fmt.Sprintf(
		`mkdir -p %s && `+
			`nohup etcd --name=%s `+
			`--initial-cluster=%s `+
			`--initial-cluster-state=%s `+
			`--listen-client-urls=%s `+
			`--listen-peer-urls=%s `+
			`--advertise-client-urls=%s `+
			`--initial-advertise-peer-urls=%s `+
			`--data-dir=%s `+
			`> /var/log/etcd.log 2>&1 &`,
		dataDir, cfg.Name, cfg.InitialCluster, cfg.State,
		cfg.ClientURL, cfg.PeerURL,
		cfg.AdvertiseClient, cfg.AdvertisePeer, dataDir,
	)
	if _, err := shell(ctx, startScript); err != nil {
		return fmt.Errorf("start etcd: %w", err)
	}

	// Don't block — etcd cluster forms asynchronously when all peers start.
	return nil
}

// ---------------------------------------------------------------------------
// installPatroni
// ---------------------------------------------------------------------------

func (e *Executor) installPatroni(ctx context.Context, cmd Command) error {
	if err := e.aptInstall(ctx, "python3-pip python3-dev libpq-dev"); err != nil {
		return fmt.Errorf("install patroni deps: %w", err)
	}
	if _, err := shell(ctx, "pip3 install patroni[etcd3] psycopg2-binary"); err != nil {
		return fmt.Errorf("install patroni: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// configPatroni
// ---------------------------------------------------------------------------

func (e *Executor) configPatroni(ctx context.Context, cmd Command) error {
	var cfg PatroniClusterConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	pgVersion := cfg.PGVersion
	if pgVersion == "" {
		pgVersion = "16"
	}

	syncMode := "false"
	if cfg.SyncMode {
		syncMode = "true"
	}

	// Build patroni.yml.
	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n")
	fmt.Fprintf(&confBuf, "scope: %s\n", cfg.Name)
	fmt.Fprintf(&confBuf, "name: %s\n\n", cfg.NodeName)

	confBuf.WriteString("restapi:\n")
	confBuf.WriteString("  listen: 0.0.0.0:8008\n")
	fmt.Fprintf(&confBuf, "  connect_address: %s:8008\n\n", cfg.ConnectAddr)

	confBuf.WriteString("etcd3:\n")
	fmt.Fprintf(&confBuf, "  hosts: %s\n\n", cfg.EtcdHosts)

	confBuf.WriteString("bootstrap:\n")
	confBuf.WriteString("  dcs:\n")
	confBuf.WriteString("    ttl: 30\n")
	confBuf.WriteString("    loop_wait: 10\n")
	confBuf.WriteString("    retry_timeout: 10\n")
	confBuf.WriteString("    maximum_lag_on_failover: 1048576\n")
	fmt.Fprintf(&confBuf, "    synchronous_mode: %s\n", syncMode)
	if cfg.SyncMode && cfg.SyncCount > 0 {
		fmt.Fprintf(&confBuf, "    synchronous_node_count: %d\n", cfg.SyncCount)
	}
	confBuf.WriteString("    postgresql:\n")
	confBuf.WriteString("      use_pg_rewind: true\n")
	confBuf.WriteString("      parameters:\n")
	confBuf.WriteString("        wal_level: replica\n")
	confBuf.WriteString("        hot_standby: 'on'\n")
	confBuf.WriteString("        max_wal_senders: 10\n")
	confBuf.WriteString("        max_replication_slots: 10\n")
	for k, v := range cfg.PGOptions {
		fmt.Fprintf(&confBuf, "        %s: '%s'\n", k, v)
	}
	confBuf.WriteString("      pg_hba:\n")
	confBuf.WriteString("        - local all all trust\n")
	confBuf.WriteString("        - host all all 0.0.0.0/0 trust\n")
	confBuf.WriteString("        - host replication all 0.0.0.0/0 trust\n")
	confBuf.WriteString("  initdb:\n")
	confBuf.WriteString("    - encoding: UTF8\n")
	confBuf.WriteString("    - data-checksums\n\n")

	dataDir := fmt.Sprintf("/var/lib/postgresql/%s/main", pgVersion)
	confBuf.WriteString("postgresql:\n")
	confBuf.WriteString("  listen: 0.0.0.0:5432\n")
	fmt.Fprintf(&confBuf, "  connect_address: %s:5432\n", cfg.ConnectAddr)
	fmt.Fprintf(&confBuf, "  data_dir: %s\n", dataDir)
	fmt.Fprintf(&confBuf, "  bin_dir: /usr/lib/postgresql/%s/bin\n", pgVersion)
	confBuf.WriteString("  pgpass: /tmp/pgpass\n")
	confBuf.WriteString("  authentication:\n")
	confBuf.WriteString("    replication:\n")
	confBuf.WriteString("      username: postgres\n")
	confBuf.WriteString("    superuser:\n")
	confBuf.WriteString("      username: postgres\n")

	confPath := "/etc/patroni/patroni.yml"
	writeScript := fmt.Sprintf("mkdir -p /etc/patroni && cat > %s << 'PATRONICFG'\n%sPATRONICFG", confPath, confBuf.String())
	if _, err := shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write patroni config: %w", err)
	}

	// Start patroni in background.
	startScript := fmt.Sprintf("mkdir -p %s && nohup patroni %s > /var/log/patroni.log 2>&1 &", dataDir, confPath)
	if _, err := shell(ctx, startScript); err != nil {
		return fmt.Errorf("start patroni: %w", err)
	}

	// Wait for patroni to be ready.
	if _, err := shell(ctx, `for i in $(seq 1 60); do curl -sf http://localhost:8008/health && break; sleep 1; done`); err != nil {
		return fmt.Errorf("patroni health check: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// installPgBouncer
// ---------------------------------------------------------------------------

func (e *Executor) installPgBouncer(ctx context.Context, cmd Command) error {
	if err := e.aptInstall(ctx, "pgbouncer"); err != nil {
		return fmt.Errorf("install pgbouncer: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// configPgBouncer
// ---------------------------------------------------------------------------

func (e *Executor) configPgBouncer(ctx context.Context, cmd Command) error {
	var cfg PgBouncerConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	// Apply defaults.
	if cfg.ListenPort == 0 {
		cfg.ListenPort = 6432
	}
	if cfg.PoolMode == "" {
		cfg.PoolMode = "transaction"
	}
	if cfg.MaxClientConn == 0 {
		cfg.MaxClientConn = 1000
	}
	if cfg.DefaultPoolSize == 0 {
		cfg.DefaultPoolSize = 25
	}
	if cfg.PGHost == "" {
		cfg.PGHost = "localhost"
	}
	if cfg.PGPort == 0 {
		cfg.PGPort = 5432
	}
	if cfg.AuthType == "" {
		cfg.AuthType = "trust"
	}

	// Write pgbouncer.ini.
	iniContent := fmt.Sprintf(`; Generated by stroppy-agent
[databases]
* = host=%s port=%d

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = %d
auth_type = %s
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = %s
max_client_conn = %d
default_pool_size = %d
admin_users = postgres
pidfile = /var/run/pgbouncer/pgbouncer.pid
logfile = /var/log/pgbouncer/pgbouncer.log
`, cfg.PGHost, cfg.PGPort, cfg.ListenPort, cfg.AuthType,
		cfg.PoolMode, cfg.MaxClientConn, cfg.DefaultPoolSize)

	iniScript := fmt.Sprintf("mkdir -p /etc/pgbouncer && cat > /etc/pgbouncer/pgbouncer.ini << 'PGBCFG'\n%sPGBCFG", iniContent)
	if _, err := shell(ctx, iniScript); err != nil {
		return fmt.Errorf("write pgbouncer.ini: %w", err)
	}

	// Write userlist.txt (trust mode, empty passwords).
	userlistScript := `cat > /etc/pgbouncer/userlist.txt << 'PGBUSR'
"postgres" ""
PGBUSR`
	if _, err := shell(ctx, userlistScript); err != nil {
		return fmt.Errorf("write pgbouncer userlist: %w", err)
	}

	startScript := `id -u pgbouncer >/dev/null 2>&1 || useradd -r -m -s /bin/false pgbouncer && ` +
		`mkdir -p /var/run/pgbouncer /var/log/pgbouncer && ` +
		`chown -R pgbouncer:pgbouncer /etc/pgbouncer /var/run/pgbouncer /var/log/pgbouncer 2>/dev/null; ` +
		`su -s /bin/bash pgbouncer -c "pgbouncer -d /etc/pgbouncer/pgbouncer.ini"`
	if _, err := shell(ctx, startScript); err != nil {
		return fmt.Errorf("start pgbouncer: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// installHAProxy
// ---------------------------------------------------------------------------

func (e *Executor) installHAProxy(ctx context.Context, cmd Command) error {
	if err := e.aptInstall(ctx, "haproxy"); err != nil {
		return fmt.Errorf("install haproxy: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// configHAProxy
// ---------------------------------------------------------------------------

func (e *Executor) configHAProxy(ctx context.Context, cmd Command) error {
	var cfg HAProxyConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	if cfg.PatroniPort == 0 {
		cfg.PatroniPort = 8008
	}

	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n")
	confBuf.WriteString("global\n")
	confBuf.WriteString("    maxconn 4096\n")
	confBuf.WriteString("    log stdout format raw local0\n\n")
	confBuf.WriteString("defaults\n")
	confBuf.WriteString("    mode tcp\n")
	confBuf.WriteString("    timeout connect 5s\n")
	confBuf.WriteString("    timeout client 30s\n")
	confBuf.WriteString("    timeout server 30s\n")
	confBuf.WriteString("    retries 3\n\n")

	// Write frontend (frontend for writes).
	fmt.Fprintf(&confBuf, "frontend ft_write\n")
	fmt.Fprintf(&confBuf, "    bind *:%d\n", cfg.WritePort)
	confBuf.WriteString("    default_backend bk_write\n\n")

	// Write frontend (frontend for reads).
	fmt.Fprintf(&confBuf, "frontend ft_read\n")
	fmt.Fprintf(&confBuf, "    bind *:%d\n", cfg.ReadPort)
	confBuf.WriteString("    default_backend bk_read\n\n")

	// Build health check options based on db kind.
	switch cfg.HealthCheck {
	case "patroni":
		// Write backend (primary via Patroni REST API).
		confBuf.WriteString("backend bk_write\n")
		fmt.Fprintf(&confBuf, "    option httpchk GET /primary\n")
		fmt.Fprintf(&confBuf, "    http-check expect status 200\n")
		confBuf.WriteString("    default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions\n")
		for i, b := range cfg.Backends {
			fmt.Fprintf(&confBuf, "    server pg%d %s check port %d\n", i, b, cfg.PatroniPort)
		}
		confBuf.WriteString("\n")

		// Read backend (replicas via Patroni REST API).
		confBuf.WriteString("backend bk_read\n")
		fmt.Fprintf(&confBuf, "    option httpchk GET /replica\n")
		fmt.Fprintf(&confBuf, "    http-check expect status 200\n")
		confBuf.WriteString("    balance roundrobin\n")
		confBuf.WriteString("    default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions\n")
		for i, b := range cfg.Backends {
			fmt.Fprintf(&confBuf, "    server pg%d %s check port %d\n", i, b, cfg.PatroniPort)
		}

	case "mysql":
		confBuf.WriteString("backend bk_write\n")
		confBuf.WriteString("    option mysql-check user haproxy\n")
		confBuf.WriteString("    default-server inter 3s fall 3 rise 2\n")
		for i, b := range cfg.Backends {
			fmt.Fprintf(&confBuf, "    server mysql%d %s check\n", i, b)
		}
		confBuf.WriteString("\n")

		confBuf.WriteString("backend bk_read\n")
		confBuf.WriteString("    option mysql-check user haproxy\n")
		confBuf.WriteString("    balance roundrobin\n")
		confBuf.WriteString("    default-server inter 3s fall 3 rise 2\n")
		for i, b := range cfg.Backends {
			fmt.Fprintf(&confBuf, "    server mysql%d %s check\n", i, b)
		}

	default: // "tcp" / picodata / fallback
		confBuf.WriteString("backend bk_write\n")
		confBuf.WriteString("    option tcp-check\n")
		confBuf.WriteString("    default-server inter 3s fall 3 rise 2\n")
		for i, b := range cfg.Backends {
			fmt.Fprintf(&confBuf, "    server srv%d %s check\n", i, b)
		}
		confBuf.WriteString("\n")

		confBuf.WriteString("backend bk_read\n")
		confBuf.WriteString("    option tcp-check\n")
		confBuf.WriteString("    balance roundrobin\n")
		confBuf.WriteString("    default-server inter 3s fall 3 rise 2\n")
		for i, b := range cfg.Backends {
			fmt.Fprintf(&confBuf, "    server srv%d %s check\n", i, b)
		}
	}

	confPath := "/etc/haproxy/haproxy.cfg"
	writeScript := fmt.Sprintf("mkdir -p /etc/haproxy && cat > %s << 'HAPCFG'\n%sHAPCFG", confPath, confBuf.String())
	if _, err := shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write haproxy config: %w", err)
	}

	// Start haproxy.
	if _, err := shell(ctx, "haproxy -f /etc/haproxy/haproxy.cfg -D"); err != nil {
		return fmt.Errorf("start haproxy: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// installProxySQL
// ---------------------------------------------------------------------------

func (e *Executor) installProxySQL(ctx context.Context, cmd Command) error {
	// Add ProxySQL apt repo and install.
	preInstall := []string{
		`wget -qO - https://repo.proxysql.com/ProxySQL/repo_pub_key | apt-key add -`,
		`echo "deb https://repo.proxysql.com/ProxySQL/proxysql-2.7.x/$(lsb_release -sc)/ ./" > /etc/apt/sources.list.d/proxysql.list`,
		`apt-get update`,
	}
	if err := e.aptPreInstall(ctx, preInstall); err != nil {
		return fmt.Errorf("proxysql repo setup: %w", err)
	}
	if err := e.aptInstall(ctx, "proxysql"); err != nil {
		return fmt.Errorf("install proxysql: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// configProxySQL
// ---------------------------------------------------------------------------

func (e *Executor) configProxySQL(ctx context.Context, cmd Command) error {
	var cfg ProxySQLConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	// Apply defaults.
	if cfg.ListenPort == 0 {
		cfg.ListenPort = 6033
	}
	if cfg.AdminPort == 0 {
		cfg.AdminPort = 6032
	}
	if cfg.WriterHostgroup == 0 {
		cfg.WriterHostgroup = 10
	}
	if cfg.ReaderHostgroup == 0 {
		cfg.ReaderHostgroup = 20
	}

	// Build proxysql.cnf.
	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n")
	confBuf.WriteString("datadir=\"/var/lib/proxysql\"\n\n")
	confBuf.WriteString("admin_variables=\n{\n")
	fmt.Fprintf(&confBuf, "    admin_credentials=\"admin:admin;radmin:radmin\"\n")
	fmt.Fprintf(&confBuf, "    mysql_ifaces=\"0.0.0.0:%d\"\n", cfg.AdminPort)
	confBuf.WriteString("}\n\n")

	confBuf.WriteString("mysql_variables=\n{\n")
	confBuf.WriteString("    threads=4\n")
	confBuf.WriteString("    max_connections=2048\n")
	confBuf.WriteString("    default_query_delay=0\n")
	confBuf.WriteString("    default_query_timeout=36000000\n")
	fmt.Fprintf(&confBuf, "    interfaces=\"0.0.0.0:%d\"\n", cfg.ListenPort)
	confBuf.WriteString("    monitor_username=\"monitor\"\n")
	confBuf.WriteString("    monitor_password=\"monitor\"\n")
	confBuf.WriteString("}\n\n")

	confBuf.WriteString("mysql_servers=\n(\n")
	for i, b := range cfg.Backends {
		// Parse host:port.
		hostgroup := cfg.WriterHostgroup
		if i > 0 {
			hostgroup = cfg.ReaderHostgroup
		}
		fmt.Fprintf(&confBuf, "    { address=\"%s\", hostgroup=%d, max_connections=100 },\n", b, hostgroup)
	}
	confBuf.WriteString(")\n\n")

	confBuf.WriteString("mysql_query_rules=\n(\n")
	fmt.Fprintf(&confBuf, "    { rule_id=1, active=1, match_digest=\"^SELECT .* FOR UPDATE$\", destination_hostgroup=%d, apply=1 },\n", cfg.WriterHostgroup)
	fmt.Fprintf(&confBuf, "    { rule_id=2, active=1, match_digest=\"^SELECT\", destination_hostgroup=%d, apply=1 },\n", cfg.ReaderHostgroup)
	confBuf.WriteString(")\n")

	confPath := "/etc/proxysql.cnf"
	writeScript := fmt.Sprintf("cat > %s << 'PSQLCFG'\n%sPSQLCFG", confPath, confBuf.String())
	if _, err := shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write proxysql config: %w", err)
	}

	// Start proxysql.
	if _, err := shell(ctx, "mkdir -p /var/lib/proxysql && proxysql --initial -f -D /var/lib/proxysql -c /etc/proxysql.cnf &"); err != nil {
		return fmt.Errorf("start proxysql: %w", err)
	}

	return nil
}
