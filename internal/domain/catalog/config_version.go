package catalog

import "strings"

// ConfigSchema resolves a role's template schema for the selected database
// version. Unversioned auxiliary components keep their own schema version.
func (d Database) ConfigSchema(version, schemaID string) string {
	if d.Kind == MySQL && strings.HasPrefix(schemaID, "cfg.my.cnf@") {
		// Schema identities omit a zero minor: MySQL 8.0 uses @8.
		return "cfg.my.cnf@" + strings.TrimSuffix(version, ".0")
	}
	if d.Kind == MariaDB && strings.HasPrefix(schemaID, "cfg.mariadb.cnf@") {
		return "cfg.mariadb.cnf@" + version
	}
	if d.Kind == Postgres && strings.HasPrefix(schemaID, "cfg.postgresql.conf@") {
		return "cfg.postgresql.conf@" + version
	}
	if d.Kind == OrioleDB && strings.HasPrefix(schemaID, "cfg.orioledb.postgresql.conf@") {
		_, major, ok := strings.Cut(version, "-pg")
		if ok {
			return "cfg.orioledb.postgresql.conf@" + major
		}
	}
	for kind, prefix := range map[DatabaseKind]string{Picodata: "cfg.picodata.yaml@", Cockroach: "cfg.cockroach.flags@", YDB: "cfg.ydb.config.yaml@"} {
		if d.Kind == kind && strings.HasPrefix(schemaID, prefix) {
			if kind == Picodata && version == "26.1" {
				return prefix + "26.1"
			}
			return prefix + strings.SplitN(version, ".", 2)[0]
		}
	}
	return schemaID
}
