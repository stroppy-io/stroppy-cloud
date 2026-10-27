package compile

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// summarize fills what the run's report shows about the setup: the tuning
// mode, the values auto tuning set, the stroppy driver, whether a commit
// waits for the disk, and per database machine the engine's cache and the
// data disk's limits. Everything is read back from
// the effective configs the containers were rendered from.
func (c *compilation) summarize() {
	effective := EffectiveWithTuned(c.in.EffectiveConfigs, c.out.Tuned)
	c.out.Tuning = run.Tuning{Mode: run.TuningMode(c.tuning()), Auto: c.out.Tuned, Client: c.clientSetup()}
	if d, ok := c.durability(effective); ok {
		c.out.Tuning.Durability = d
	}
	for i := range c.out.Machines {
		snap := &c.out.Machines[i]
		h := c.hostOf(snap.Name)
		snap.DiskReadIOPS, snap.DiskWriteIOPS = h.Disk.ReadIOPS, h.Disk.WriteIOPS
		snap.DiskReadMBps, snap.DiskWriteMBps = h.Disk.ReadMBps, h.Disk.WriteMBps
		snap.CacheMB = c.cacheMB(effective[snap.Role], h.MemoryMB)
	}
}

// engineConfig is one key of a role's config, wherever its schema version.
func engineConfig(byID map[string]json.RawMessage, prefix, key string) (any, bool) {
	for id, raw := range byID {
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		values := map[string]any{}
		if json.Unmarshal(raw, &values) != nil {
			continue
		}
		v, ok := values[key]
		return v, ok && v != nil
	}
	return nil, false
}

// cacheMB is the engine's cache of a role's machine in MB.
func (c *compilation) cacheMB(byID map[string]json.RawMessage, memoryMB int) int {
	switch c.in.Database.Kind { //nolint:exhaustive // YDB splits its cache by percent of a hard limit; the rest have none
	case catalog.Postgres:
		v, _ := engineConfig(byID, "cfg.postgresql.conf@", "shared_buffers")
		return int(numberOf(v))
	case catalog.OrioleDB:
		v, _ := engineConfig(byID, "cfg.orioledb.postgresql.conf@", "orioledb_main_buffers")
		return int(numberOf(v))
	case catalog.MySQL:
		v, _ := engineConfig(byID, "cfg.my.cnf@", "innodb_buffer_pool_size")
		return int(numberOf(v))
	case catalog.MariaDB:
		v, _ := engineConfig(byID, "cfg.mariadb.cnf@", "innodb_buffer_pool_size")
		return int(numberOf(v))
	case catalog.Cockroach:
		v, _ := engineConfig(byID, "cfg.cockroach.flags@", "cache")
		s, _ := v.(string) //nolint:errcheck // absent = unknown
		return sizeMB(s, memoryMB)
	case catalog.Picodata:
		v, _ := engineConfig(byID, "cfg.picodata.yaml@", "memtx_memory")
		s, _ := v.(string) //nolint:errcheck // absent = unknown
		return sizeMB(s, memoryMB)
	default:
		return 0
	}
}

var sizeRe = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(%|[KMGT]i?B?|)$`)

// sizeMB reads an engine's size string: a share of RAM ("35%", ".25") or
// an amount ("8GiB", "2048M", "20G"); 0 when unreadable.
func sizeMB(s string, memoryMB int) int {
	s = strings.TrimSpace(s)
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 && f < 1 {
		return int(f * float64(memoryMB))
	}
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseFloat(m[1], 64) //nolint:errcheck // matched digits
	switch unit := strings.TrimSuffix(strings.TrimSuffix(m[2], "B"), "i"); unit {
	case "%":
		return int(n * float64(memoryMB) / 100)
	case "K":
		return int(n / 1024)
	case "", "M":
		return int(n)
	case "G":
		return int(n * 1024)
	case "T":
		return int(n * 1024 * 1024)
	}
	return 0
}

// durability is what a commit of the database waits for, read from the
// primary's config; false when the engine gives no single answer.
func (c *compilation) durability(effective map[string]map[string]json.RawMessage) (run.CommitDurability, bool) {
	db := effective["db"]
	on := func(v any, ok bool, want ...string) bool {
		if !ok {
			return true // the schema default is the engine's durable one
		}
		s := strings.ToLower(strings.TrimSpace(strings.Trim(stringOf(v), "'\"")))
		for _, w := range want {
			if s == w {
				return true
			}
		}
		return false
	}
	verdict := func(durable bool) (run.CommitDurability, bool) {
		if durable {
			return run.DurabilityFsync, true
		}
		return run.DurabilityRelaxed, true
	}
	switch c.in.Database.Kind { //nolint:exhaustive // noop, external, managed and custom have no answer here
	case catalog.Postgres, catalog.OrioleDB:
		prefix := "cfg.postgresql.conf@"
		if c.in.Database.Kind == catalog.OrioleDB {
			prefix = "cfg.orioledb.postgresql.conf@"
		}
		fsync, okF := engineConfig(db, prefix, "fsync")
		commit, okC := engineConfig(db, prefix, "synchronous_commit")
		return verdict(on(fsync, okF, "on", "true") && on(commit, okC, "on", "local", "remote_write", "remote_apply"))
	case catalog.MySQL, catalog.MariaDB:
		prefix := "cfg.my.cnf@"
		if c.in.Database.Kind == catalog.MariaDB {
			prefix = "cfg.mariadb.cnf@"
		}
		flush, okF := engineConfig(db, prefix, "innodb_flush_log_at_trx_commit")
		binlog, okB := engineConfig(db, prefix, "sync_binlog")
		logBin, okL := engineConfig(db, prefix, "log_bin")
		binlogOff := okL && (stringOf(logBin) == "" || stringOf(logBin) == "false" || stringOf(logBin) == "OFF")
		return verdict(on(flush, okF, "1") && (binlogOff || on(binlog, okB, "1")))
	case catalog.Picodata:
		// wal_mode exists from 26.2; before it the WAL is written without fsync.
		for _, raw := range db {
			if strings.Contains(string(raw), `"wal_mode":"fsync"`) {
				return run.DurabilityFsync, true
			}
		}
		return run.DurabilityRelaxed, true
	case catalog.Cockroach, catalog.YDB:
		// Both sync their log (Raft / PDisk) before acknowledging a commit.
		return run.DurabilityFsync, true
	default:
		return "", false
	}
}

// stringOf prints a config value.
func stringOf(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		b, _ := json.Marshal(v) //nolint:errcheck // plain values
		return string(b)
	}
}
