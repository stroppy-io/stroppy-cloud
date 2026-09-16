#!/usr/bin/env python3
"""Read database/replication state from an isolated MySQL test VM over TCP.

Requires PyMySQL. Uses the test recipe's database account, not Docker or sudo.
The local test server has a self-signed certificate; TLS encrypts the connection
but certificate verification is disabled for this loopback-only probe.
"""
import json
import os
import ssl

import pymysql


def main():
    tls = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    tls.check_hostname = False
    tls.verify_mode = ssl.CERT_NONE
    connection = pymysql.connect(
        host="127.0.0.1", port=3306, user="root",
        password=os.environ.get("MYSQL_PWD", "stroppy_mysql"),
        ssl=tls, connect_timeout=10, read_timeout=10,
        cursorclass=pymysql.cursors.DictCursor,
    )
    with connection, connection.cursor() as cursor:
        queries = {
            "server": "SELECT VERSION() AS version, @@server_id AS server_id, @@read_only AS read_only, @@datadir AS datadir",
            "plugins": "SELECT PLUGIN_NAME, PLUGIN_STATUS FROM information_schema.PLUGINS WHERE PLUGIN_NAME LIKE 'rpl_semi_sync%'",
            "semisync": "SHOW GLOBAL STATUS LIKE 'Rpl_semi_sync%'",
            "settings": "SHOW GLOBAL VARIABLES LIKE 'rpl_semi_sync%'",
            "replication": "SHOW REPLICA STATUS",
            "replicated_rows": "SELECT COUNT(*) AS rows_present FROM stroppy.stroppy_demo",
        }
        result = {}
        for name, sql in queries.items():
            cursor.execute(sql)
            result[name] = cursor.fetchall()
        print(json.dumps(result, default=str))


if __name__ == "__main__":
    main()
