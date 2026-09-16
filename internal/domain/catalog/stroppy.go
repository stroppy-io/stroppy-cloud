package catalog

// stroppy is the v0 static stroppy catalog: one build, the scripts stroppy
// 6.0.0 registers (`stroppy probe -o json`), their steps and typed flags.
// Later sources (release catalog.json, probe) replace this table.
func stroppy() StroppyCatalog {
	all := []Protocol{ProtoPg, ProtoMySQL, ProtoPicodata, ProtoYDBGrpc, ProtoYDBGrpcs, ProtoCockroach, ProtoNoop}
	sql := []Protocol{ProtoPg, ProtoMySQL, ProtoPicodata, ProtoCockroach}
	procs := []Protocol{ProtoPg, ProtoMySQL, ProtoCockroach}
	tpccSteps := []StroppyStep{
		{ID: "drop_schema", Title: "Drop schema", Phase: "bootstrap"},
		{ID: "create_schema", Title: "Create schema", Phase: "bootstrap"},
		{ID: "load_data", Title: "Load data", Phase: "bootstrap"},
		{ID: "workload_tx_new_order", Title: "New order", Phase: "workload"},
		{ID: "workload_tx_payment", Title: "Payment", Phase: "workload"},
		{ID: "workload_tx_order_status", Title: "Order status", Phase: "workload"},
		{ID: "workload_tx_delivery", Title: "Delivery", Phase: "workload"},
		{ID: "workload_tx_stock_level", Title: "Stock level", Phase: "workload"},
		{ID: "workload_mixed", Title: "TPC-C mix", Phase: "workload"},
	}
	tpccParams := []StroppyParam{
		{Name: "scale-factor", Config: "scaleFactor", Type: "int", Default: 1, Description: "Number of warehouses.", Env: "SCALE_FACTOR"},
		{Name: "warehouse-start", Config: "warehouseStart", Type: "int", Default: 1, Description: "First warehouse id to load."},
		{Name: "load-items", Config: "loadItems", Type: "bool", DefaultDescription: "true when warehouse-start is 1; false otherwise"},
		{Name: "load-workers", Config: "loadWorkers", Type: "int", Default: 8, Description: "Parallel loaders."},
		{Name: "duration", Config: "duration", Scope: "run", Type: "duration", Default: "0s", Description: "Workload duration; 0 = until iterations end."},
		{Name: "vus", Config: "vus", Scope: "run", Type: "int", Default: 1, Description: "Virtual users."},
	}
	tpcbSteps := []StroppyStep{
		{ID: "drop_schema", Phase: "bootstrap"},
		{ID: "create_schema", Phase: "bootstrap"},
		{ID: "load_data", Phase: "bootstrap"},
		{ID: "workload_tx", Title: "TPC-B transaction", Phase: "workload"},
	}
	tpcbParams := []StroppyParam{
		{Name: "scale-factor", Config: "scaleFactor", Type: "int", Default: 1, Description: "Branches (×100 000 accounts)."},
		{Name: "duration", Config: "duration", Scope: "run", Type: "duration", Default: "0s"},
		{Name: "vus", Config: "vus", Scope: "run", Type: "int", Default: 1},
	}
	return StroppyCatalog{
		Source: "static",
		Versions: []StroppyVersion{{
			Version: "6.0.0", Image: "ghcr.io/stroppy-io/stroppy:v6.0.0.62", Default: true, Baseline: true, Protocols: all,
			Scripts: []StroppyScript{
				{ID: "tpcc/tx", Title: "TPC-C, raw transactions", Description: "TPC-C with every transaction issued as client-side SQL.", Protocols: all, Steps: tpccSteps, Params: tpccParams},
				{ID: "tpcc/procs", Title: "TPC-C, stored procedures", Description: "TPC-C with the transaction logic in server-side procedures.", Protocols: procs, Steps: tpccSteps, Params: tpccParams},
				{ID: "tpcb/tx", Title: "TPC-B, raw transactions", Protocols: all, Steps: tpcbSteps, Params: tpcbParams},
				{ID: "tpcb/procs", Title: "TPC-B, stored procedures", Protocols: procs, Steps: tpcbSteps, Params: tpcbParams},
				{
					ID: "tpch/tx", Title: "TPC-H", Description: "Relational load of eight tables plus the 22-query suite.", Protocols: sql,
					Steps: []StroppyStep{{ID: "drop_schema", Phase: "bootstrap"}, {ID: "create_schema", Phase: "bootstrap"}, {ID: "load_data", Phase: "bootstrap"}, {ID: "workload_queries", Title: "Query suite", Phase: "workload"}},
					Params: []StroppyParam{
						{Name: "scale-factor", Config: "scaleFactor", Type: "float64", Default: 1.0, Description: "TPC-H scale factor (1 ≈ 1 GB)."},
						{Name: "streams", Config: "streams", Type: "int", Default: 1, Description: "Query streams."},
						{Name: "query-seed", Config: "querySeed", Type: "int64", Default: 19620718},
					},
				},
				{
					ID: "tpcds", Title: "TPC-DS", Protocols: sql,
					Steps:  []StroppyStep{{ID: "drop_schema", Phase: "bootstrap"}, {ID: "create_schema", Phase: "bootstrap"}, {ID: "load_data", Phase: "bootstrap"}, {ID: "workload_queries", Phase: "workload"}},
					Params: []StroppyParam{{Name: "scale-factor", Config: "scaleFactor", Type: "float64", Default: 1.0}},
				},
				{
					ID: "simple", Title: "Simple key-value", Description: "Point reads and writes on one table.", Protocols: all,
					Steps:  []StroppyStep{{ID: "drop_schema", Phase: "bootstrap"}, {ID: "create_schema", Phase: "bootstrap"}, {ID: "load_data", Phase: "bootstrap"}, {ID: "workload_mixed", Phase: "workload"}},
					Params: []StroppyParam{{Name: "scale-factor", Config: "scaleFactor", Type: "int", Default: 1}, {Name: "vus", Config: "vus", Scope: "run", Type: "int", Default: 1}, {Name: "duration", Config: "duration", Scope: "run", Type: "duration", Default: "0s"}},
				},
				{
					ID: "execute_sql", Title: "Execute SQL", Description: "Run supplied SQL statements. The noop driver measures client execution overhead without a database.", Protocols: all,
					Steps:  []StroppyStep{{ID: "execute", Title: "Execute", Phase: "workload"}},
					Params: []StroppyParam{{Name: "sql-file", Config: "sqlFile", Type: "string", Description: "File shipped next to the config."}},
				},
			},
		}},
	}
}
