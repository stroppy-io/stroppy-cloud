package workload

import (
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"
)

// doc: Stroppy pkg/config/config.go PoolConfig, PostgresConfig and SQLConfig.
// Keep explicit zeros and unset values distinct: zero disables caches/idle
// connections/lifetimes in the upstream driver.
func driverPool(name, title string, fields ...schemapb.FieldDef) *schemapb.ObjectB {
	return schemapb.Object(schemapb.FieldName(name), fields...).Title(title).Group("Driver").Desc("Native Stroppy " + name + " options; explicit driver settings take precedence over pool aliases.").Strict()
}

func connectionCount(name, title string) schemapb.FieldDef {
	return schemapb.Int64(schemapb.FieldName(name)).Title(title).Group("Pool").Desc("Stroppy " + name + "; unset preserves the driver default.").Gte(0).Lte(65535).Nullable()
}

func connectionDuration(name, title string) schemapb.FieldDef {
	return schemapb.Duration(schemapb.FieldName(name)).Title(title).Group("Pool").Desc("Stroppy " + name + "; zero disables the lifetime limit.").Gte(0).Lte(24 * time.Hour).Nullable()
}

func postgresFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		connectionCount("max_conns", "Max connections"), connectionCount("min_conns", "Min connections"), connectionCount("min_idle_conns", "Min idle connections"),
		connectionDuration("max_conn_lifetime", "Max lifetime"), connectionDuration("max_conn_idle_time", "Max idle time"),
		connectionCount("description_cache_capacity", "Description cache"), connectionCount("statement_cache_capacity", "Statement cache"),
		schemapb.Choice("trace_log_level").Title("Driver log level").Group("Logging").Desc("pgx tracer log level (traceLogLevel).").Opt(schemapb.StrV("trace"), "Trace").Opt(schemapb.StrV("debug"), "Debug").Opt(schemapb.StrV("info"), "Info").Opt(schemapb.StrV("warn"), "Warn").Opt(schemapb.StrV("error"), "Error").Opt(schemapb.StrV("none"), "None").Nullable(),
		schemapb.Choice("default_query_exec_mode").Title("Query execution mode").Group("Driver").Desc("pgx defaultQueryExecMode.").Opt(schemapb.StrV("cache_statement"), "Cache statements").Opt(schemapb.StrV("cache_describe"), "Cache descriptions").Opt(schemapb.StrV("describe_exec"), "Describe and execute").Opt(schemapb.StrV("exec"), "Execute").Opt(schemapb.StrV("simple_protocol"), "Simple protocol").Nullable(),
	}
}

func sqlFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		connectionCount("max_open_conns", "Max open connections"), connectionCount("max_idle_conns", "Max idle connections"),
		connectionDuration("conn_max_lifetime", "Max lifetime"), connectionDuration("conn_max_idle_time", "Max idle time"),
	}
}

func poolFields() []schemapb.FieldDef { return append(postgresFields(), sqlFields()...) }
