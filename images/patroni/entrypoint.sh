#!/bin/sh
set -eu
mkdir -p "$PGDATA" /var/run/postgresql
chown postgres:postgres "$PGDATA" /var/lib/postgresql/data /var/run/postgresql
chmod 700 "$PGDATA"
exec gosu postgres patroni "$@"
