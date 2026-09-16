#!/bin/sh
set -eu
name=graphene-disk-postgres
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT
mkdir -p /data/postgres
# No published ports; trust is limited to this disposable container.
docker run -d --name "$name" --network none -e POSTGRES_HOST_AUTH_METHOD=trust -v /data/postgres:/var/lib/postgresql/data postgres:17
ready() {
  n=0
  until docker exec "$name" pg_isready -U postgres >/dev/null 2>&1; do
    n=$((n+1)); test "$n" -lt 60; sleep 1
  done
}
ready
docker exec "$name" psql -U postgres -v ON_ERROR_STOP=1 -c "CREATE TABLE disk_proof AS SELECT generate_series(1,10000) AS n;"
docker restart "$name" >/dev/null
ready
test "$(docker exec "$name" psql -U postgres -Atc 'SELECT count(*) FROM disk_proof')" = 10000
docker exec "$name" psql -U postgres -Atc 'SHOW data_directory'
docker inspect "$name" --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{end}}'
findmnt --target /data/postgres
cleanup
umount /data
/bin/sh /tmp/mount-data.sh
test "$(cat /data/runtime-proof)" = disk-persistence
printf 'PASS: PostgreSQL retained 10000 rows across restart; disk remount retained marker\n'
