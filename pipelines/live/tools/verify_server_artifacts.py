#!/usr/bin/env python3
"""Download run artifacts through the server and verify size/digest without saving contents."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
from urllib.parse import quote
from urllib.request import Request, urlopen

from server_api import Client


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tenant", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--evidence", required=True)
    args = parser.parse_args()
    client = Client()
    path = f"/api/v1/t/{quote(args.tenant, safe='')}/runs/{quote(args.run_id, safe='')}/artifacts"
    listing = client.request(path)
    if listing["http_status"] != 200:
        raise RuntimeError(f"artifact list: HTTP {listing['http_status']}")
    results = []
    for artifact in listing["body"]["data"]:
        req = Request(client.base + path + "/" + quote(artifact["id"], safe=""),
                      headers={"Authorization": "Bearer " + client.token})
        digest, size = hashlib.sha256(), 0
        with urlopen(req, timeout=120) as response:
            while chunk := response.read(1024 * 1024):
                digest.update(chunk)
                size += len(chunk)
            status = response.status
        actual = "sha256:" + digest.hexdigest()
        results.append({"id": artifact["id"], "http_status": status,
                        "size_bytes": size, "digest": actual,
                        "verified": size == artifact["size_bytes"] and actual == artifact["digest"]})
    report = {"observed_at": datetime.now(timezone.utc).isoformat(), "run_id": args.run_id,
              "verified": bool(results) and all(r["verified"] for r in results), "artifacts": results}
    dest = Path(args.evidence)
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))
    return 0 if report["verified"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
