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
	}

	// Always add monitor and stroppy.
	cfg.Machines = append(cfg.Machines,
		types.MachineSpec{Role: types.RoleMonitor, Count: 1, CPUs: 1, MemoryMB: 2048, DiskGB: 20},
		types.MachineSpec{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20},
	)
}
