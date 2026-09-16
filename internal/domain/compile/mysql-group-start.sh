#!/usr/bin/env bash
set -euo pipefail
export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"
env -u MYSQL_PWD docker-entrypoint.sh mysqld &
server=$!
trap 'kill -TERM "$server" 2>/dev/null || true; wait "$server" || true' EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
ready=0
for ((i=0; i<300; i++)); do
    kill -0 "$server" 2>/dev/null || exit 1
    if mysql --ssl-mode=REQUIRED -h127.0.0.1 -uroot -e 'SELECT 1' >/dev/null 2>&1; then
        ready=1
        break
    fi
    sleep 1
done
[ "$ready" = 1 ] || { echo 'cluster-init: MySQL readiness timed out' >&2; exit 1; }
mysql --ssl-mode=REQUIRED -h127.0.0.1 -uroot <<SQL
CHANGE REPLICATION SOURCE TO SOURCE_USER='stroppy_recovery', SOURCE_PASSWORD='$STROPPY_RECOVERY_PASSWORD' FOR CHANNEL 'group_replication_recovery';
SQL
bootstrap=OFF
marker=/var/lib/mysql/.stroppy-cluster-bootstrap
if [ "$STROPPY_CLUSTER_SEED" = 1 ] && [ ! -e "$marker" ]; then
    # Mark BEFORE bootstrap: an interrupted attempt must never form a second group.
    touch "$marker"
    bootstrap=ON
fi
echo "cluster-init: starting Group Replication (bootstrap=$bootstrap)"
mysql --ssl-mode=REQUIRED -h127.0.0.1 -uroot <<SQL
SET GLOBAL group_replication_bootstrap_group=$bootstrap;
START GROUP_REPLICATION;
SET GLOBAL group_replication_bootstrap_group=OFF;
SQL
wait "$server"
