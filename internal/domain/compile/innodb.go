package compile

import "encoding/json"

// autoBufferPoolShare is the share of a database machine's RAM the InnoDB
// buffer pool takes in the "auto" mode: MySQL suggests up to ~80% on a
// dedicated server, the rest leaves room for connections and the OS.
const autoBufferPoolShare = 0.7

// innodbMemory sizes the InnoDB buffer pool of one MySQL/MariaDB server by
// the database's buffer_pool mode (db.<kind>.params), unless the role's
// config sets it explicitly. "auto" also lifts MariaDB's I/O capacity from
// its 200 IOPS (spinning disk) default to the one cfg.my.cnf uses.
func (c *compilation) innodbMemory(role, schemaID, machine string, maria bool, overlay map[string]any) {
	p := c.params()
	explicit := c.explicitConfig(role, schemaID)
	switch strParam(p, "buffer_pool", "auto") {
	case "auto":
		if gb := c.memoryOf(machine); gb > 0 && !explicit["innodb_buffer_pool_size"] {
			overlay["innodb_buffer_pool_size"] = int(float64(gb*1024) * autoBufferPoolShare)
		}
	case "fixed":
		if !explicit["innodb_buffer_pool_size"] {
			overlay["innodb_buffer_pool_size"] = intParam(p, "buffer_pool_mb", 4096)
		}
	default: // stock: the server's own defaults
		return
	}
	if maria {
		if !explicit["innodb_io_capacity"] {
			overlay["innodb_io_capacity"] = 10000
		}
		if !explicit["innodb_io_capacity_max"] {
			overlay["innodb_io_capacity_max"] = 20000
		}
	}
}

// explicitConfig is the keys the database sets in a role's config (its
// deviations from the schema defaults).
func (c *compilation) explicitConfig(role, schemaID string) map[string]bool {
	out := map[string]bool{}
	if rc, ok := c.in.Database.Configs[role]; ok {
		values := map[string]json.RawMessage{}
		if json.Unmarshal(rc[schemaID], &values) == nil {
			for k := range values {
				out[k] = true
			}
		}
	}
	return out
}

// memoryOf is a machine's RAM in GB (0 when unknown).
func (c *compilation) memoryOf(machine string) int {
	for _, m := range c.out.Spec.Machines {
		if m.Name == machine {
			return m.MemoryGB
		}
	}
	return 0
}
