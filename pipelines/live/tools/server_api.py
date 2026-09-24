#!/usr/bin/env python3
"""Exercise Stroppy Cloud through HTTP and retain response evidence (no request secrets)."""

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import sys
import urllib.error
import urllib.request


class Client:
    def __init__(self, base=None, token=None):
        self.base = (base or os.environ.get("STROPPY_TEST_URL", "http://localhost:18347")).rstrip("/")
        self.token = token or os.environ.get("STROPPY_TEST_TOKEN", "dev")

    def request(self, path, method="GET", data=None, evidence=None, timeout=120):
        if not path.startswith("/api/v1/"):
            raise ValueError("use an absolute Stroppy Cloud API path")
        # Minted API tokens are readable once and must never become public evidence.
        if evidence and "/tokens" in path:
            raise ValueError("token endpoints cannot be recorded as public evidence")
        body = None if data is None else json.dumps(data).encode()
        req = urllib.request.Request(self.base + path, data=body, method=method,
                                     headers={"Authorization": "Bearer " + self.token,
                                              "Content-Type": "application/json"})
        try:
            response = urllib.request.urlopen(req, timeout=timeout)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            try:
                value = json.loads(raw) if raw else None
            except ValueError:
                value = raw.decode(errors="replace")
            record = {"observed_at": datetime.now(timezone.utc).isoformat(),
                      "method": method, "path": path, "http_status": response.code,
                      "contract_version": response.headers.get("X-Stroppy-Contract-Version"),
                      "body": value}
        if evidence:
            dest = Path(evidence)
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_text(json.dumps(record, indent=2, ensure_ascii=False) + "\n")
        return record


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path")
    parser.add_argument("--method", default="GET")
    parser.add_argument("--data", help="JSON request file, or - for stdin; never recorded")
    parser.add_argument("--evidence", help="public response evidence file (exclude secret-returning endpoints)")
    args = parser.parse_args()
    data = None
    if args.data:
        data = json.load(sys.stdin) if args.data == "-" else json.loads(Path(args.data).read_text())
    record = Client().request(args.path, args.method, data, args.evidence)
    print(json.dumps(record, indent=2, ensure_ascii=False))
    return 0 if record["http_status"] < 400 else 1


if __name__ == "__main__":
    sys.exit(main())
