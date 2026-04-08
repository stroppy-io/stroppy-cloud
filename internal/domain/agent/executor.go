package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// LogCallback is invoked for every output line produced by shell commands.
// commandID identifies the originating command; line is the raw text;
// stream is "stdout" (combined stdout+stderr).
type LogCallback func(commandID, line, stream string)

// Executor runs agent commands on the local machine.
// Long-running processes (DB servers, exporters, vmagent) are tracked in a
// pool and killed on Shutdown(). Their stdout/stderr are streamed via logCallback.
type Executor struct {
	aptMu        sync.Mutex
	bootstrapMu  sync.Once
	bootstrapErr error

	logMu       sync.RWMutex
	logCallback LogCallback
	currentCmd  string

	// Process pool — tracked background processes killed on shutdown.
	procMu sync.Mutex
	procs  []*managedProc
}

type managedProc struct {
	name string
	cmd  *exec.Cmd
	stop func()
}

// NewExecutor returns a new Executor.
func NewExecutor() *Executor { return &Executor{} }

// Shutdown kills all tracked background processes.
func (e *Executor) Shutdown() {
	e.procMu.Lock()
	defer e.procMu.Unlock()
	for _, p := range e.procs {
		if p.cmd.Process != nil {
			p.cmd.Process.Kill()
		}
		if p.stop != nil {
			p.stop()
		}
	}
	e.procs = nil
}

// startDaemon launches a long-running process, tracks it in the pool,
// and streams its output through logCallback. Returns immediately.
func (e *Executor) startDaemon(name string, binPath string, args ...string) error {
	cmd := exec.Command(binPath, args...)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		pw.Close()
		return fmt.Errorf("start %s: %w", name, err)
	}

	// Stream output in background.
	go func() {
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			e.emitLine(scanner.Text())
		}
	}()

	// Wait for process in background — when it dies, close pipe.
	stopCh := make(chan struct{})
	go func() {
		cmd.Wait()
		pw.Close()
		close(stopCh)
	}()

	e.procMu.Lock()
	e.procs = append(e.procs, &managedProc{
		name: name,
		cmd:  cmd,
		stop: func() { <-stopCh },
	})
	e.procMu.Unlock()

	return nil
}

// SetLogCallback registers a callback that receives every shell output line in real-time.
func (e *Executor) SetLogCallback(cb LogCallback) {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	e.logCallback = cb
}

// setCurrentCommand stores the currently executing command ID for log correlation.
func (e *Executor) setCurrentCommand(id string) {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	e.currentCmd = id
}

// emitLine sends a single output line to the registered callback (if any).
func (e *Executor) emitLine(line string) {
	// Always print to stderr so `docker logs` captures everything.
	fmt.Fprintln(os.Stderr, line)

	e.logMu.RLock()
	cb := e.logCallback
	cmdID := e.currentCmd
	e.logMu.RUnlock()
	if cb != nil {
		cb(cmdID, line, "stdout")
	}
}

// bootstrap installs base utilities required by all actions. Runs once, thread-safe.
func (e *Executor) bootstrap(ctx context.Context) error {
	e.bootstrapMu.Do(func() {
		// Prevent services from auto-starting during apt install (Docker has no systemd).
		// Best-effort: failure here is non-fatal (e.g. read-only filesystem), but worth logging.
		if _, err := e.shell(ctx, `printf '#!/bin/sh\nexit 101\n' > /usr/sbin/policy-rc.d && chmod +x /usr/sbin/policy-rc.d`); err != nil {
			log.Printf("bootstrap: policy-rc.d setup failed (non-fatal): %v", err)
		}
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
	return e.shell(ctx, script)
}

// aptInstall runs apt-get install with the apt lock held to prevent concurrent apt operations.
func (e *Executor) aptInstall(ctx context.Context, packages string) error {
	_, err := e.shellWithAptLock(ctx, "DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "+packages)
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

// installPackage installs a Package on the target machine:
// 1. Custom repo + GPG key (if set)
// 2. Pre-install commands (shell)
// 3. .deb file download + dpkg -i (if deb_url set)
// 4. apt-get install (if apt_packages set)
func (e *Executor) installPackage(ctx context.Context, pkg types.Package) error {
	// 1. Add custom repo if specified
	if pkg.CustomRepo != "" {
		if pkg.CustomRepoKey != "" {
			if _, err := e.shellWithAptLock(ctx, fmt.Sprintf(
				`curl -fsSL "%s" | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/custom.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/custom.gpg`,
				pkg.CustomRepoKey)); err != nil {
				return fmt.Errorf("add custom repo key: %w", err)
			}
		}
		if _, err := e.shellWithAptLock(ctx, fmt.Sprintf(
			`echo "%s" > /etc/apt/sources.list.d/custom.list && apt-get update`,
			pkg.CustomRepo)); err != nil {
			return fmt.Errorf("add custom repo: %w", err)
		}
	}

	// 2. Pre-install commands
	if len(pkg.PreInstall) > 0 {
		if err := e.aptPreInstall(ctx, pkg.PreInstall); err != nil {
			return fmt.Errorf("pre-install: %w", err)
		}
	}

	// 3. .deb file (downloaded via URL injected by server at run start)
	if pkg.DebFilename != "" {
		debURL := pkg.DebFilename // server has replaced filename with download URL
		debPath := "/tmp/custom_package.deb"
		// Build curl command with auth header if token is provided.
		curlAuth := ""
		if pkg.DebToken != "" {
			curlAuth = fmt.Sprintf(` -H "Authorization: Bearer %s"`, pkg.DebToken)
		}
		// Use apt-get install (not dpkg -i) so dependencies are resolved automatically.
		script := fmt.Sprintf(`curl -fsSL%s "%s" -o %s && DEBIAN_FRONTEND=noninteractive apt-get install -y %s`, curlAuth, debURL, debPath, debPath)
		if _, err := e.shellWithAptLock(ctx, script); err != nil {
			return fmt.Errorf("install deb %s: %w", debURL, err)
		}
	}

	// 4. apt packages
	if len(pkg.AptPackages) > 0 {
		if err := e.aptInstall(ctx, strings.Join(pkg.AptPackages, " ")); err != nil {
			return fmt.Errorf("apt install: %w", err)
		}
	}

	return nil
}

// Run executes a Command and returns a Report.
func (e *Executor) Run(ctx context.Context, cmd Command) Report {
	report := Report{CommandID: cmd.ID, Status: ReportCompleted}

	// Store command ID so streamed log lines are correlated.
	e.setCurrentCommand(cmd.ID)
	defer e.setCurrentCommand("")

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

// shell runs a bash script, streaming each output line to the executor's
// logCallback in real-time while also accumulating the full output.
func (e *Executor) shell(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", script)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var fullOutput strings.Builder

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		// Allow lines up to 1 MB (apt-get progress, curl, etc.).
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			fullOutput.WriteString(line + "\n")
			e.emitLine(line)
		}
	}()

	if err := cmd.Start(); err != nil {
		pw.Close()
		return "", err
	}

	cmdErr := cmd.Wait()
	pw.Close()
	<-done

	output := fullOutput.String()
	if cmdErr != nil {
		return output, fmt.Errorf("%w: %s", cmdErr, output)
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
	if cfg.Package == nil {
		return fmt.Errorf("no package provided for postgres installation")
	}
	if err := e.installPackage(ctx, *cfg.Package); err != nil {
		return fmt.Errorf("install postgres: %w", err)
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

	// Mandatory streaming replication params (required for pg_basebackup when Patroni is disabled).
	// Set before user Options so these act as non-overridable base values.
	if !cfg.Patroni {
		defaults["wal_level"] = "replica"
		defaults["max_wal_senders"] = "10"
		defaults["max_replication_slots"] = "10"
		if cfg.Role == "replica" {
			defaults["hot_standby"] = "on"
		}
	}

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
	if _, err := e.shell(ctx, writeScript); err != nil {
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
	if _, err := e.shell(ctx, hbaScript); err != nil {
		return fmt.Errorf("write pg_hba.conf: %w", err)
	}

	if cfg.Role == "master" {
		startScript := fmt.Sprintf(`pg_ctlcluster %s main start || true
# Wait for postgres to be ready.
for i in $(seq 1 30); do
  pg_isready -U postgres && break
  sleep 1
done`, version)
		if _, err := e.shell(ctx, startScript); err != nil {
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

		if _, err := e.shell(ctx, replicaScript); err != nil {
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
	if cfg.Package == nil {
		return fmt.Errorf("no package provided for mysql installation")
	}
	// MySQL postinst tries to start/stop mysqld which fails in Docker.
	// Install with error tolerance, then verify binary exists.
	if err := e.installPackage(ctx, *cfg.Package); err != nil {
		if _, verr := e.shell(ctx, "which mysqld"); verr != nil {
			return fmt.Errorf("install mysql: %w", err)
		}
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

	// --- LOCKED params: server-id (unique per node) ---
	defaults["server-id"] = fmt.Sprintf("%d", cfg.NodeIndex+1)

	// --- Mandatory GTID replication params ---
	defaults["gtid_mode"] = "ON"
	defaults["enforce_gtid_consistency"] = "ON"
	defaults["log_bin"] = "mysql-bin"

	// --- Semi-Sync replication ---
	if cfg.SemiSync {
		if cfg.Role == "primary" {
			defaults["plugin-load-add"] = "semisync_source.so"
			defaults["rpl_semi_sync_source_enabled"] = "1"
		} else {
			defaults["plugin-load-add"] = "semisync_replica.so"
			defaults["rpl_semi_sync_replica_enabled"] = "1"
		}
	}

	// --- Group Replication ---
	if cfg.GroupRepl {
		defaults["binlog_format"] = "ROW"
		defaults["binlog_checksum"] = "NONE"
		defaults["transaction_write_set_extraction"] = "XXHASH64"
		if cfg.GroupName != "" {
			defaults["group_replication_group_name"] = cfg.GroupName
		}
		localHost := cfg.LocalHost
		if localHost == "" {
			localHost = cfg.PrimaryHost
		}
		defaults["group_replication_local_address"] = fmt.Sprintf("%s:33061", localHost)
		defaults["group_replication_group_seeds"] = strings.Join(cfg.GroupSeeds, ",")
		defaults["group_replication_single_primary_mode"] = "ON"
		defaults["group_replication_start_on_boot"] = "OFF"
	}

	// User Options applied last (can override tunable params but not locked ones like server-id).
	for k, v := range cfg.Options {
		if k == "server-id" {
			continue // locked
		}
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
	if _, err := e.shell(ctx, writeScript); err != nil {
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
	if _, err := e.shell(ctx, initScript); err != nil {
		return fmt.Errorf("init mysql: %w", err)
	}

	if err := e.startDaemon("mysqld", "mysqld",
		"--defaults-file=/etc/mysql/my.cnf",
		"--datadir="+dataDir,
		"--user=mysql",
	); err != nil {
		return fmt.Errorf("start mysql: %w", err)
	}
	// Wait for mysqld to be ready.
	e.shell(ctx, `for i in $(seq 1 30); do mysqladmin ping -h 127.0.0.1 -u root --silent 2>/dev/null && break; sleep 1; done`)

	if cfg.Role == "primary" {
		// Create replication user on primary so replicas can connect.
		replUserScript := `mysql -h 127.0.0.1 -u root -e "CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED BY 'repl_password'; GRANT REPLICATION SLAVE ON *.* TO 'repl'@'%'; FLUSH PRIVILEGES;"`
		if _, err := e.shell(ctx, replUserScript); err != nil {
			return fmt.Errorf("create replication user: %w", err)
		}
		// Create ProxySQL monitor user on primary.
		monitorUserScript := `mysql -h 127.0.0.1 -u root -e "CREATE USER IF NOT EXISTS 'monitor'@'%' IDENTIFIED BY 'monitor'; GRANT USAGE ON *.* TO 'monitor'@'%'; FLUSH PRIVILEGES;"`
		if _, err := e.shell(ctx, monitorUserScript); err != nil {
			return fmt.Errorf("create monitor user: %w", err)
		}
	}

	// Group Replication bootstrap/join.
	if cfg.GroupRepl {
		if cfg.Role == "primary" {
			grBootstrap := `mysql -h 127.0.0.1 -u root -e "SET GLOBAL group_replication_bootstrap_group=ON; START GROUP_REPLICATION; SET GLOBAL group_replication_bootstrap_group=OFF;"`
			if _, err := e.shell(ctx, grBootstrap); err != nil {
				return fmt.Errorf("bootstrap group replication: %w", err)
			}
		} else {
			// Wait for primary GR to be ready before joining.
			waitPrimary := fmt.Sprintf(`for i in $(seq 1 30); do mysql -h %s -u repl -prepl_password -e "SELECT 1" 2>/dev/null && break; sleep 2; done`,
				cfg.PrimaryHost)
			if _, err := e.shell(ctx, waitPrimary); err != nil {
				return fmt.Errorf("wait for mysql primary (GR): %w", err)
			}
			grJoin := `mysql -h 127.0.0.1 -u root -e "START GROUP_REPLICATION;"`
			if _, err := e.shell(ctx, grJoin); err != nil {
				return fmt.Errorf("join group replication: %w", err)
			}
		}
	} else if cfg.Role == "replica" && cfg.PrimaryHost != "" {
		// Standard async/semi-sync replication.
		// Wait for primary to be reachable before configuring replication.
		waitPrimary := fmt.Sprintf(`for i in $(seq 1 30); do mysql -h %s -u repl -prepl_password -e "SELECT 1" 2>/dev/null && break; sleep 2; done`,
			cfg.PrimaryHost)
		if _, err := e.shell(ctx, waitPrimary); err != nil {
			return fmt.Errorf("wait for mysql primary: %w", err)
		}
		replicaScript := fmt.Sprintf(`mysql -h 127.0.0.1 -u root -e "CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_USER='repl', SOURCE_PASSWORD='repl_password', SOURCE_AUTO_POSITION=1; START REPLICA;"`,
			cfg.PrimaryHost)
		if _, err := e.shell(ctx, replicaScript); err != nil {
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
	if cfg.Package == nil {
		return fmt.Errorf("no package provided for picodata installation")
	}
	if err := e.installPackage(ctx, *cfg.Package); err != nil {
		return fmt.Errorf("install picodata: %w", err)
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
	resolveMemoryDefaults(defaults)
	for k, v := range cfg.Options {
		defaults[k] = v
	}

	replication := cfg.Replication
	if replication < 1 {
		replication = 2
	}

	// The first peer is the bootstrap instance; all others join via it.
	firstPeer := cfg.Peers[0]
	if !strings.Contains(firstPeer, ":") {
		firstPeer = firstPeer + ":3301"
	}

	memtxMemory := defaults["memtx_memory"]
	// Convert MB suffix to bytes for picodata YAML config.
	memtxMemory = strings.TrimSuffix(memtxMemory, "MB")
	var memtxBytes int
	fmt.Sscanf(memtxMemory, "%d", &memtxBytes)
	if memtxBytes > 0 {
		memtxBytes = memtxBytes * 1024 * 1024
	} else {
		memtxBytes = 256 * 1024 * 1024
	}

	// Build proper picodata YAML config.
	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n")
	confBuf.WriteString("cluster:\n")
	confBuf.WriteString("  name: stroppy-cluster\n")
	confBuf.WriteString("  tier:\n")
	confBuf.WriteString("    default:\n")
	fmt.Fprintf(&confBuf, "      replication_factor: %d\n", replication)
	confBuf.WriteString("      can_vote: true\n")
	confBuf.WriteString("\n")
	confBuf.WriteString("instance:\n")
	fmt.Fprintf(&confBuf, "  name: instance-%d\n", cfg.InstanceID)
	confBuf.WriteString("  tier: default\n")
	confBuf.WriteString("  peer:\n")
	fmt.Fprintf(&confBuf, "    - %s\n", firstPeer)
	confBuf.WriteString("  iproto:\n")
	confBuf.WriteString("    listen: 0.0.0.0:3301\n")
	confBuf.WriteString("  pgproto:\n")
	confBuf.WriteString("    listen: 0.0.0.0:4327\n")
	confBuf.WriteString("  http:\n")
	confBuf.WriteString("    listen: 0.0.0.0:8081\n")
	confBuf.WriteString("  memtx:\n")
	fmt.Fprintf(&confBuf, "    memory: %d\n", memtxBytes)

	confPath := "/etc/picodata/picodata.yaml"
	writeScript := fmt.Sprintf("mkdir -p /etc/picodata && cat > %s << 'PICOCONF'\n%sPICOCONF", confPath, confBuf.String())
	if _, err := e.shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write picodata config: %w", err)
	}

	// Start picodata.
	dataDir := "/var/lib/picodata"
	e.shell(ctx, fmt.Sprintf("mkdir -p %s", dataDir))
	if err := e.startDaemon("picodata", "picodata", "run", "--config", confPath); err != nil {
		return fmt.Errorf("start picodata: %w", err)
	}
	// Wait for picodata readiness.
	_, startErr := e.shell(ctx, `for i in $(seq 1 30); do curl -sf http://localhost:8081/api/v1/health/ready 2>/dev/null && break; sleep 2; done`)
	if startErr != nil {
		return fmt.Errorf("picodata readiness: %w", startErr)
	}

	return nil
}

// ---------------------------------------------------------------------------
// installMonitor
// ---------------------------------------------------------------------------

func (e *Executor) installMonitor(ctx context.Context, cmd Command) error {
	var cfg MonitorInstallConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}

	mon := types.DefaultMonitoring()
	machineID := os.Getenv("STROPPY_MACHINE_ID")

	// --- node_exporter on ALL machines ---
	neVer := mon.NodeExporterVersion
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
	if _, err := e.shell(ctx, neScript); err != nil {
		return fmt.Errorf("install node_exporter: %w", err)
	}

	// --- postgres_exporter only on database machines (and only for postgres) ---
	if strings.Contains(machineID, "-database-") && cfg.DatabaseKind == "postgres" {
		peVer := mon.PostgresExporterVersion
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
		if _, err := e.shell(ctx, peScript); err != nil {
			return fmt.Errorf("install postgres_exporter: %w", err)
		}
	}

	// --- vmagent on every machine (scrapes local exporters, pushes to VictoriaMetrics) ---
	{
		vaVer := mon.VmagentVersion
		if vaVer == "" {
			vaVer = "1.139.0"
		}
		vaURL := fmt.Sprintf(
			"https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v%s/vmutils-linux-amd64-v%s.tar.gz",
			vaVer, vaVer,
		)
		vaScript := fmt.Sprintf(
			`curl -fsSL "%s" -o /tmp/vmutils.tar.gz && `+
				`tar xzf /tmp/vmutils.tar.gz -C /tmp && `+
				`cp /tmp/vmagent-prod /usr/local/bin/vmagent && `+
				`chmod +x /usr/local/bin/vmagent && `+
				`rm -rf /tmp/vmutils* /tmp/vmagent* /tmp/vmalert* /tmp/vmauth* /tmp/vmbackup* /tmp/vmrestore*`,
			vaURL,
		)
		if _, err := e.shell(ctx, vaScript); err != nil {
			return fmt.Errorf("install vmagent: %w", err)
		}
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

	machineID := os.Getenv("STROPPY_MACHINE_ID")

	// --- node_exporter on EVERY machine ---
	if err := e.startDaemon("node_exporter", "/usr/local/bin/node_exporter"); err != nil {
		return fmt.Errorf("start node_exporter: %w", err)
	}

	// --- postgres_exporter on database machines (connects to LOCAL postgres) ---
	if strings.Contains(machineID, "-database-") && cfg.DatabaseKind == "postgres" {
		os.Setenv("DATA_SOURCE_NAME", "postgresql://postgres@localhost:5432/postgres?sslmode=disable")
		if err := e.startDaemon("postgres_exporter", "/usr/local/bin/postgres_exporter"); err != nil {
			// Non-fatal: postgres_exporter may fail transiently if PG is still starting.
			log.Printf("WARNING: postgres_exporter failed to start: %v", err)
		}
	}

	// --- vmagent on every machine (scrapes local exporters, pushes to VictoriaMetrics) ---
	{
		var confBuf strings.Builder
		confBuf.WriteString("# Generated by stroppy-agent\n")
		confBuf.WriteString("global:\n  scrape_interval: 5s\n")

		if cfg.RunID != "" {
			fmt.Fprintf(&confBuf, "  external_labels:\n    stroppy_run_id: '%s'\n", cfg.RunID)
		}

		confBuf.WriteString("\nscrape_configs:\n")

		// node_exporter on localhost.
		confBuf.WriteString("  - job_name: node\n    static_configs:\n      - targets: ['localhost:9100']\n")

		// DB exporter on localhost (only on database machines).
		if strings.Contains(machineID, "-database-") {
			switch cfg.DatabaseKind {
			case "postgres":
				confBuf.WriteString("  - job_name: postgres\n    static_configs:\n      - targets: ['localhost:9187']\n")
			}
		}

		confPath := "/etc/vmagent/scrape.yml"
		writeScript := fmt.Sprintf("mkdir -p /etc/vmagent && cat > %s << 'PROMCFG'\n%sPROMCFG", confPath, confBuf.String())
		if _, err := e.shell(ctx, writeScript); err != nil {
			return fmt.Errorf("write vmagent scrape config: %w", err)
		}

		remoteWrite := cfg.MetricsEndpoint

		e.shell(ctx, "mkdir -p /var/lib/vmagent")
		vmagentArgs := []string{
			"-promscrape.config=" + confPath,
			"-remoteWrite.url=" + remoteWrite,
			"-remoteWrite.tmpDataPath=/var/lib/vmagent",
		}
		if cfg.BearerToken != "" {
			vmagentArgs = append(vmagentArgs, "-remoteWrite.bearerToken="+cfg.BearerToken)
		}
		if err := e.startDaemon("vmagent", "/usr/local/bin/vmagent", vmagentArgs...); err != nil {
			return fmt.Errorf("start vmagent: %w", err)
		}
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
		// Fallback matches DefaultStroppySettings().Version.
		version = types.DefaultStroppySettings().Version
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
	if _, err := e.shell(ctx, script); err != nil {
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

	// Write stroppy-config.json and run via -f flag.
	configPath := "/tmp/stroppy-config.json"
	if err := os.WriteFile(configPath, []byte(cfg.ConfigJSON), 0644); err != nil {
		return fmt.Errorf("write stroppy config: %w", err)
	}

	script := fmt.Sprintf("stroppy run -f %s", configPath)
	if _, err := e.shell(ctx, script); err != nil {
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
		// Fallback matches DefaultMonitoring().EtcdVersion.
		version = types.DefaultMonitoring().EtcdVersion
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
	if _, err := e.shell(ctx, script); err != nil {
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
	if _, err := e.shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write etcd env: %w", err)
	}

	dataDir := "/var/lib/etcd"
	e.shell(ctx, fmt.Sprintf("mkdir -p %s", dataDir))
	if err := e.startDaemon("etcd", "/usr/local/bin/etcd",
		"--name="+cfg.Name,
		"--initial-cluster="+cfg.InitialCluster,
		"--initial-cluster-state="+cfg.State,
		"--listen-client-urls="+cfg.ClientURL,
		"--listen-peer-urls="+cfg.PeerURL,
		"--advertise-client-urls="+cfg.AdvertiseClient,
		"--initial-advertise-peer-urls="+cfg.AdvertisePeer,
		"--data-dir="+dataDir,
	); err != nil {
		return fmt.Errorf("start etcd: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// installPatroni
// ---------------------------------------------------------------------------

func (e *Executor) installPatroni(ctx context.Context, cmd Command) error {
	if err := e.aptInstall(ctx, "python3-pip python3-dev libpq-dev"); err != nil {
		return fmt.Errorf("install patroni deps: %w", err)
	}
	if _, err := e.shell(ctx, "pip3 install patroni[etcd3] psycopg2-binary"); err != nil {
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
	if _, err := e.shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write patroni config: %w", err)
	}

	e.shell(ctx, fmt.Sprintf("mkdir -p %s", dataDir))
	if err := e.startDaemon("patroni", "patroni", confPath); err != nil {
		return fmt.Errorf("start patroni: %w", err)
	}

	// Wait for patroni to be ready.
	if _, err := e.shell(ctx, `for i in $(seq 1 60); do curl -sf http://localhost:8008/health && break; sleep 1; done`); err != nil {
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
	if _, err := e.shell(ctx, iniScript); err != nil {
		return fmt.Errorf("write pgbouncer.ini: %w", err)
	}

	// Write userlist.txt (trust mode, empty passwords).
	userlistScript := `cat > /etc/pgbouncer/userlist.txt << 'PGBUSR'
"postgres" ""
PGBUSR`
	if _, err := e.shell(ctx, userlistScript); err != nil {
		return fmt.Errorf("write pgbouncer userlist: %w", err)
	}

	startScript := `id -u pgbouncer >/dev/null 2>&1 || useradd -r -m -s /bin/false pgbouncer && ` +
		`mkdir -p /var/run/pgbouncer /var/log/pgbouncer && ` +
		`chown -R pgbouncer:pgbouncer /etc/pgbouncer /var/run/pgbouncer /var/log/pgbouncer 2>/dev/null; ` +
		`su -s /bin/bash pgbouncer -c "pgbouncer -d /etc/pgbouncer/pgbouncer.ini"`
	if _, err := e.shell(ctx, startScript); err != nil {
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
	if _, err := e.shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write haproxy config: %w", err)
	}

	// Start haproxy.
	if _, err := e.shell(ctx, "haproxy -f /etc/haproxy/haproxy.cfg -D"); err != nil {
		return fmt.Errorf("start haproxy: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// installProxySQL
// ---------------------------------------------------------------------------

func (e *Executor) installProxySQL(ctx context.Context, cmd Command) error {
	// Download ProxySQL from GitHub releases (more reliable than repo.proxysql.com).
	version := "2.7.3"
	url := fmt.Sprintf("https://github.com/sysown/proxysql/releases/download/v%s/proxysql_%s-ubuntu22_amd64.deb", version, version)
	githubScript := fmt.Sprintf(`curl -fsSL "%s" -o /tmp/proxysql.deb && dpkg -i /tmp/proxysql.deb && rm -f /tmp/proxysql.deb`, url)

	if _, err := e.shell(ctx, githubScript); err != nil {
		// Fallback: use apt repo if GitHub download fails.
		preInstall := []string{
			`wget -qO - https://repo.proxysql.com/ProxySQL/repo_pub_key | apt-key add -`,
			`echo "deb https://repo.proxysql.com/ProxySQL/proxysql-2.7.x/$(lsb_release -sc)/ ./" > /etc/apt/sources.list.d/proxysql.list`,
			`apt-get update`,
		}
		if err := e.aptPreInstall(ctx, preInstall); err != nil {
			return fmt.Errorf("proxysql repo setup (fallback): %w", err)
		}
		if err := e.aptInstall(ctx, "proxysql"); err != nil {
			return fmt.Errorf("install proxysql (fallback): %w", err)
		}
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
	if _, err := e.shell(ctx, writeScript); err != nil {
		return fmt.Errorf("write proxysql config: %w", err)
	}

	// Start proxysql.
	if _, err := e.shell(ctx, "mkdir -p /var/lib/proxysql && proxysql --initial -f -D /var/lib/proxysql -c /etc/proxysql.cnf &"); err != nil {
		return fmt.Errorf("start proxysql: %w", err)
	}

	return nil
}
