#!/usr/bin/env bash
set -euo pipefail
export MYSQL_PWD="$MARIADB_ROOT_PASSWORD"
# The official entrypoint's temporary server must not join Galera while
# installing system tables. Initialize once with wsrep explicitly disabled.
if [ ! -d /var/lib/mysql/mysql ]; then
    echo 'cluster-init: initializing local datadir with wsrep disabled'
    env -u MYSQL_PWD docker-entrypoint.sh mariadbd --wsrep-on=OFF --wsrep-provider=none &
    server=$!
    trap 'kill -TERM "$server" 2>/dev/null || true; wait "$server" || true' EXIT
    trap 'exit 143' TERM
    trap 'exit 130' INT
    ready=0
    for ((i=0; i<300; i++)); do
        kill -0 "$server" 2>/dev/null || exit 1
        if mariadb -h127.0.0.1 -uroot -e 'SELECT 1' >/dev/null 2>&1; then
            ready=1
            break
        fi
        sleep 1
    done
    [ "$ready" = 1 ] || { echo 'cluster-init: MariaDB initialization timed out' >&2; exit 1; }
    mariadb-admin -h127.0.0.1 -uroot shutdown
    wait "$server"
    trap - EXIT TERM INT
fi
marker=/var/lib/mysql/.stroppy-cluster-bootstrap
if [ "$STROPPY_CLUSTER_SEED" = 1 ] && [ ! -e "$marker" ]; then
    # Do not automatically bootstrap again after an interrupted attempt.
    touch "$marker"
    echo 'cluster-init: bootstrapping the initial Galera component'
    exec env -u MYSQL_PWD docker-entrypoint.sh mariadbd --wsrep-new-cluster
fi
echo 'cluster-init: joining the existing Galera component'
exec env -u MYSQL_PWD docker-entrypoint.sh mariadbd
