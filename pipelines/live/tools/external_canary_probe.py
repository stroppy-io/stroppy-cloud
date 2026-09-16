#!/usr/bin/env python3
"""Read the owned external-test fixture through its trusted loopback listener."""
import argparse
import json
import socket
import struct

from picodata_probe import receive, send


def inspect(port=5432, database="stroppy"):
    if not database or "\0" in database:
        raise ValueError("Invalid fixture database name")
    with socket.create_connection(("127.0.0.1", port), timeout=15) as sock:
        send(sock, b"", struct.pack("!I", 196608)
             + b"user\0external_reader\0database\0" + database.encode() + b"\0\0")
        while True:
            kind, body = receive(sock)
            if kind == b"E":
                raise RuntimeError("Fixture rejected the loopback reader session")
            if kind == b"R" and struct.unpack("!I", body[:4])[0] != 0:
                raise RuntimeError("This probe requires the fixture's trusted loopback listener")
            if kind == b"Z":
                break
        query = """SELECT current_user, current_database(), count(*), min(id),
            has_table_privilege(current_user, 'external_fixture_canary', 'SELECT'),
            has_table_privilege(current_user, 'external_fixture_canary', 'INSERT'),
            has_table_privilege(current_user, 'external_fixture_canary', 'UPDATE'),
            has_table_privilege(current_user, 'external_fixture_canary', 'DELETE'),
            has_table_privilege(current_user, 'external_fixture_canary', 'TRUNCATE')
            FROM external_fixture_canary;"""
        send(sock, b"Q", query.encode() + b"\0")
        rows = []
        while True:
            kind, body = receive(sock)
            if kind == b"E":
                raise RuntimeError("Fixture canary query failed")
            if kind == b"D":
                count = struct.unpack("!H", body[:2])[0]
                offset, row = 2, []
                for _ in range(count):
                    size = struct.unpack("!i", body[offset:offset + 4])[0]
                    offset += 4
                    row.append(None if size == -1 else body[offset:offset + size].decode())
                    if size != -1:
                        offset += size
                rows.append(row)
            if kind == b"Z":
                break
        send(sock, b"X", b"")
    if rows != [["external_reader", database, "1", "197", "t", "f", "f", "f", "f"]]:
        raise RuntimeError("Canary contents or reader table privileges changed")
    return {"probe": "external_canary", "status": "passed", "canary_rows": 1,
            "canary_id": 197, "database": database, "reader_select": True, "reader_table_writes": False,
            "source": "read-only PostgreSQL query over the owned fixture's loopback listener"}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=5432)
    parser.add_argument("--database", default="stroppy")
    args = parser.parse_args()
    print(json.dumps(inspect(args.port, args.database)), flush=True)
