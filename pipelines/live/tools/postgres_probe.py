#!/usr/bin/env python3
"""Probe an isolated test VM over local trust auth; --seed writes 100 test rows."""
import json
import sys
import socket
import struct


def exact(sock, n):
    out = b""
    while len(out) < n:
        chunk = sock.recv(n - len(out))
        if not chunk:
            raise RuntimeError("PostgreSQL closed the connection")
        out += chunk
    return out


def message(sock):
    kind = exact(sock, 1)
    size = struct.unpack("!I", exact(sock, 4))[0]
    return kind, exact(sock, size - 4)


def main():
    with socket.create_connection(("127.0.0.1", 5432), timeout=10) as sock:
        startup = struct.pack("!I", 196608) + b"user\0postgres\0database\0postgres\0application_name\0stroppy_readonly_probe\0\0"
        sock.sendall(struct.pack("!I", len(startup) + 4) + startup)
        while True:
            kind, body = message(sock)
            if kind == b"E":
                raise RuntimeError(body.decode(errors="replace"))
            if kind == b"R" and body != struct.pack("!I", 0):
                raise RuntimeError("Probe requires the test recipe's local trust auth")
            if kind == b"Z":
                break
        queries = [
            "SELECT current_setting('server_version'), pg_is_in_recovery(), current_setting('data_directory')",
            ("CREATE TABLE IF NOT EXISTS public.stroppy_replication_probe (id integer primary key); "
             "INSERT INTO public.stroppy_replication_probe SELECT generate_series(1,100) ON CONFLICT DO NOTHING; "
             "SELECT count(*) FROM public.stroppy_replication_probe" if "--seed" in sys.argv
             else "SELECT count(*) FROM public.stroppy_replication_probe"),
            "SELECT status, latest_end_lsn::text, slot_name, sender_host FROM pg_stat_wal_receiver",
            "SELECT state, sync_state, sent_lsn::text, replay_lsn::text FROM pg_stat_replication",
        ]
        if "--orioledb" in sys.argv:
            queries.extend([
                "SELECT extversion, current_setting('default_table_access_method'), current_setting('server_encoding'), (SELECT datcollate FROM pg_database WHERE datname=current_database()) FROM pg_extension WHERE extname='orioledb'",
                "SELECT n.nspname, c.relname, am.amname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_am am ON am.oid=c.relam WHERE c.relkind='r' AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' ORDER BY 1,2",
                "SELECT name, setting, source FROM pg_settings WHERE name IN ('shared_preload_libraries','pg_stat_statements.max','pg_stat_statements.track','orioledb.undo_buffers') ORDER BY name",
                "SELECT count(*) FROM pg_stat_statements",
            ])
        for sql in queries:
            payload = sql.encode() + b"\0"
            sock.sendall(b"Q" + struct.pack("!I", len(payload) + 4) + payload)
            rows = []
            while True:
                kind, body = message(sock)
                if kind == b"E":
                    raise RuntimeError(body.decode(errors="replace"))
                if kind == b"D":
                    count = struct.unpack("!H", body[:2])[0]
                    pos = 2
                    row = []
                    for _ in range(count):
                        size = struct.unpack("!i", body[pos:pos + 4])[0]
                        pos += 4
                        row.append(None if size == -1 else body[pos:pos + size].decode())
                        if size != -1:
                            pos += size
                    rows.append(row)
                if kind == b"Z":
                    break
            print(json.dumps({"query": sql, "rows": rows}), flush=True)


if __name__ == "__main__":
    main()
