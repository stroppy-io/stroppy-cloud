#!/usr/bin/env python3
"""Read version and gossip membership on each owned CockroachDB test node."""
import argparse
import json
import socket
import struct
import urllib.request

from picodata_probe import receive, send


def query(sock, sql):
    send(sock, b"Q", sql.encode() + b"\0")
    rows = []
    while True:
        kind, body = receive(sock)
        if kind == b"E":
            raise RuntimeError(body.decode(errors="replace"))
        if kind == b"D":
            count = struct.unpack("!H", body[:2])[0]
            offset = 2
            row = []
            for _ in range(count):
                length = struct.unpack("!i", body[offset:offset + 4])[0]
                offset += 4
                row.append(None if length == -1 else body[offset:offset + length].decode())
                if length != -1:
                    offset += length
            rows.append(row)
        if kind == b"Z":
            return rows


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--members", type=int, required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    with socket.create_connection((args.host, 26257), timeout=15) as sock:
        send(sock, b"", struct.pack("!I", 196608) + b"user\0root\0database\0defaultdb\0\0")
        while True:
            kind, body = receive(sock)
            if kind == b"E":
                raise RuntimeError(body.decode(errors="replace"))
            if kind == b"R" and struct.unpack("!I", body[:4])[0] != 0:
                raise RuntimeError("This probe requires the compiled insecure test-node authentication")
            if kind == b"Z":
                break
        version = query(sock, "SELECT version()")[0][0]
        send(sock, b"X", b"")
    if " v" + args.version + "." not in version:
        raise RuntimeError("Unexpected version: " + version)
    # Use the node status API; recent versions restrict crdb_internal tables.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open("http://" + args.host + ":8080/health?ready=1", timeout=15) as response:
        response.read()
    with opener.open("http://" + args.host + ":8080/_status/nodes", timeout=15) as response:
        nodes = json.load(response)["nodes"]
    members = [{"id": n["desc"]["nodeId"], "address": n["desc"]["address"]["addressField"], "version": n["buildInfo"]["tag"]} for n in nodes]
    if len(members) != args.members or len({r["id"] for r in members}) != args.members:
        raise RuntimeError("Unexpected membership: " + repr(members))
    if not all(r["version"].startswith("v" + args.version + ".") for r in members):
        raise RuntimeError("Member version mismatch: " + repr(members))
    print(json.dumps({"probe": "cockroach", "status": "passed", "version": version, "members": members, "local_node_ready": True}), flush=True)



if __name__ == "__main__":
    main()
