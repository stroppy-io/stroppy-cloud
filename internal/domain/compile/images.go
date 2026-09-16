package compile

// Images of the sidecars the recipes add next to the databases. Database
// images come from the catalog versions; these are the fixed companions.
//
// doc: hub.docker.com — official/verified tags, pinned to a minor line.
const (
	imageHAProxy         = "haproxy:3.0"
	imagePgBouncer       = "edoburu/pgbouncer:v1.24.1-p1"
	imagePgBouncerExport = "quay.io/prometheuscommunity/pgbouncer-exporter:v0.12.1"
	imageProxySQL        = "proxysql/proxysql:2.7.3"
	imageNodeExporter    = "quay.io/prometheus/node-exporter:v1.12.1"
	imagePostgresExport  = "quay.io/prometheuscommunity/postgres-exporter:v0.18.1"
	imageMySQLDExporter  = "prom/mysqld-exporter:v0.19.0"
)
