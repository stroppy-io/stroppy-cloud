package run

import "github.com/stroppy-io/stroppy-cloud/internal/domain/types"

// fillMachinesFromTopology populates cfg.Machines from the database topology
// when the caller (e.g. SPA) did not specify machines explicitly.
func FillMachinesFromTopology(cfg *types.RunConfig) {
	if len(cfg.Machines) > 0 {
		return // user specified machines explicitly
	}

	db := cfg.Database
	switch db.Kind {
	case types.DatabasePostgres:
		if db.Postgres != nil {
			dbCount := db.Postgres.Master.Count
			for _, r := range db.Postgres.Replicas {
				dbCount += r.Count
			}
			cfg.Machines = append(cfg.Machines, types.MachineSpec{Role: types.RoleDatabase, Count: dbCount, CPUs: db.Postgres.Master.CPUs, MemoryMB: db.Postgres.Master.MemoryMB, DiskGB: db.Postgres.Master.DiskGB})
			if db.Postgres.HAProxy != nil {
				cfg.Machines = append(cfg.Machines, *db.Postgres.HAProxy)
			}
		}
	case types.DatabaseMySQL:
		if db.MySQL != nil {
			dbCount := db.MySQL.Primary.Count
			for _, r := range db.MySQL.Replicas {
				dbCount += r.Count
			}
			cfg.Machines = append(cfg.Machines, types.MachineSpec{Role: types.RoleDatabase, Count: dbCount, CPUs: db.MySQL.Primary.CPUs, MemoryMB: db.MySQL.Primary.MemoryMB, DiskGB: db.MySQL.Primary.DiskGB})
			if db.MySQL.ProxySQL != nil {
				cfg.Machines = append(cfg.Machines, *db.MySQL.ProxySQL)
			}
		}
	case types.DatabasePicodata:
		if db.Picodata != nil {
			for _, inst := range db.Picodata.Instances {
				cfg.Machines = append(cfg.Machines, inst)
			}
			if db.Picodata.HAProxy != nil {
				cfg.Machines = append(cfg.Machines, *db.Picodata.HAProxy)
			}
		}
	case types.DatabaseYDB:
		if db.YDB != nil {
			// In combined mode (Database==nil), storage nodes run both storage + database processes.
			// In split mode, each node still runs both — we use the larger count and best specs.
			// True split (separate machines for storage vs database) requires a new machine role.
			count := db.YDB.Storage.Count
			cpus := db.YDB.Storage.CPUs
			mem := db.YDB.Storage.MemoryMB
			disk := db.YDB.Storage.DiskGB
			if db.YDB.Database != nil {
				if db.YDB.Database.Count > count {
					count = db.YDB.Database.Count
				}
				if db.YDB.Database.CPUs > cpus {
					cpus = db.YDB.Database.CPUs
				}
				if db.YDB.Database.MemoryMB > mem {
					mem = db.YDB.Database.MemoryMB
				}
				if db.YDB.Database.DiskGB > disk {
					disk = db.YDB.Database.DiskGB
				}
			}
			cfg.Machines = append(cfg.Machines, types.MachineSpec{
				Role: types.RoleDatabase, Count: count,
				CPUs: cpus, MemoryMB: mem, DiskGB: disk,
			})
			if db.YDB.HAProxy != nil {
				cfg.Machines = append(cfg.Machines, *db.YDB.HAProxy)
			}
		}
	}

	// Add stroppy runner — use custom spec if provided, otherwise default.
	stroppySpec := types.MachineSpec{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20}
	if cfg.Stroppy.Machine != nil {
		stroppySpec = *cfg.Stroppy.Machine
		stroppySpec.Role = types.RoleStroppy
		stroppySpec.Count = 1
	}
	cfg.Machines = append(cfg.Machines, stroppySpec)
}
