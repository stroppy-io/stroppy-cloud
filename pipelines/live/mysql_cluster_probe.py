#!/usr/bin/env python3
"""Read all cluster members and a replicated test table using ordinary SQL."""
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
        password=os.environ.get("MYSQL_PWD", "stroppy_mysql"), ssl=tls,
        connect_timeout=10, read_timeout=10, cursorclass=pymysql.cursors.DictCursor,
    )
    with connection, connection.cursor() as cursor:
        cursor.execute("SELECT VERSION() AS version, @@server_id AS server_id, @@read_only AS read_only, @@datadir AS datadir")
        result = {"server": cursor.fetchall()}
        maria = "MariaDB" in result["server"][0]["version"]
        if maria:
            cursor.execute("SHOW GLOBAL STATUS LIKE 'wsrep_%'")
            result["wsrep"] = cursor.fetchall()
            status = {r["Variable_name"]: r["Value"] for r in result["wsrep"]}
            assert status["wsrep_cluster_size"] == "3"
            assert status["wsrep_cluster_status"] == "Primary"
            assert status["wsrep_local_state"] == "4" and status["wsrep_ready"] == "ON"
        else:
            cursor.execute("SELECT MEMBER_ID,MEMBER_HOST,MEMBER_PORT,MEMBER_STATE,MEMBER_ROLE,MEMBER_VERSION FROM performance_schema.replication_group_members")
            result["members"] = cursor.fetchall()
            assert len(result["members"]) == 3 and all(m["MEMBER_STATE"] == "ONLINE" for m in result["members"])
            cursor.execute("SELECT CHANNEL_NAME,WORKER_ID,SERVICE_STATE,LAST_ERROR_NUMBER,LAST_ERROR_MESSAGE FROM performance_schema.replication_applier_status_by_worker")
            result["appliers"] = cursor.fetchall()
            assert all(m["LAST_ERROR_NUMBER"] == 0 for m in result["appliers"])
        cursor.execute("SELECT COUNT(*) AS rows_present FROM stroppy.stroppy_demo")
        result["replicated_rows"] = cursor.fetchall()
        assert result["replicated_rows"][0]["rows_present"] == 100
        print(json.dumps(result, default=str))


if __name__ == "__main__":
    main()
