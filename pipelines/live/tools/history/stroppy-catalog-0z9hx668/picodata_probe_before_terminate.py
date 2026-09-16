#!/usr/bin/env python3
"""Read Picodata membership through pgwire on an owned test stand."""
import argparse
import hashlib
import json
import os
import socket
import struct


def exact(sock, size):
    result = b""
    while len(result) < size:
        chunk = sock.recv(size - len(result))
        if not chunk:
            raise RuntimeError("Picodata closed the connection")
        result += chunk
    return result


def receive(sock):
    kind = exact(sock, 1)
    size = struct.unpack("!I", exact(sock, 4))[0]
    if size < 4 or size > 16 * 1024 * 1024:
        raise RuntimeError("Invalid pgwire message length")
    return kind, exact(sock, size - 4)


def send(sock, kind, payload):
    sock.sendall(kind + struct.pack("!I", len(payload) + 4) + payload)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--members", required=True, type=int)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    password = os.environ["PGPASSWORD"]
    with socket.create_connection((args.host, 4327), timeout=15) as sock:
        startup = struct.pack("!I", 196608) + b"user\0admin\0database\0postgres\0\0"
        send(sock, b"", startup)
        while True:
            kind, body = receive(sock)
            if kind == b"E":
                raise RuntimeError(body.decode(errors="replace"))
            if kind == b"R":
                method = struct.unpack("!I", body[:4])[0]
                if method == 3:
                    send(sock, b"p", password.encode() + b"\0")
                elif method == 5:
                    inner = hashlib.md5((password + "admin").encode()).hexdigest().encode()
                    response = b"md5" + hashlib.md5(inner + body[4:8]).hexdigest().encode()
                    send(sock, b"p", response + b"\0")
                elif method != 0:
                    raise RuntimeError("Unsupported pgwire authentication method " + str(method))
            if kind == b"Z":
                break
        send(sock, b"Q", b"SELECT name, current_state, picodata_version FROM _pico_instance;\0")
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
                break
        if len(rows) != args.members or len({r[0] for r in rows}) != args.members:
            raise RuntimeError("Unexpected cluster membership: " + repr(rows))
        if not all(json.loads(r[1])[0] == "Online" and r[2].startswith(args.version + ".") for r in rows):
            raise RuntimeError("Member state or version mismatch: " + repr(rows))
        print(json.dumps({"probe": "picodata", "status": "passed", "members": rows}), flush=True)


if __name__ == "__main__":
    main()
