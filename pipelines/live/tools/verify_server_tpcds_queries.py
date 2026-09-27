#!/usr/bin/env python3
"""Verify every generated TPC-DS query ID in the complete server log artifact."""
import argparse
import json
from pathlib import Path
import re
from urllib.parse import quote
from urllib.request import Request, urlopen

from server_api import Client


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--tenant", required=True)
    p.add_argument("--run-id", required=True)
    p.add_argument("--segment", default="tpcds-generated")
    p.add_argument("--evidence", required=True)
    a = p.parse_args()
    c = Client()
    base = f"/api/v1/t/{a.tenant}/runs/{a.run_id}"
    r = c.request(base)
    assert r["http_status"] == 200 and r["body"]["status"] == "completed"
    segment = next(s for s in r["body"]["result"]["segments"] if s["name"] == a.segment)
    assert segment["status"] == "completed" and segment["metrics"]["failed_queries_total"]["value"] == 0
    listing = c.request(base+"/artifacts")
    assert listing["http_status"] == 200
    artifact = next(x for x in listing["body"]["data"] if x["id"].endswith("-stroppy-"+a.segment+"-log"))
    req = Request(c.base+base+"/artifacts/"+quote(artifact["id"], safe=""), headers={"Authorization": "Bearer "+c.token})
    counts = {}
    pattern = re.compile(r"\[tpcds\] query(\d+)(?:_[abc])?: ok")
    with urlopen(req, timeout=120) as response:
        for raw in response:
            for q in pattern.findall(raw.decode(errors="replace")):
                counts[int(q)] = counts.get(int(q), 0)+1
    missing = sorted(set(range(1, 100))-set(counts))
    unexpected = sorted(set(counts)-set(range(1, 100)))
    report = {"run_id": a.run_id, "segment": a.segment, "artifact_id": artifact["id"],
              "query_ids": sorted(counts), "statement_counts": counts,
              "missing_queries": missing, "unexpected_queries": unexpected,
              "statement_count": sum(counts.values()),
              "verified": not missing and not unexpected and sum(counts.values()) == 103}
    Path(a.evidence).write_text(json.dumps(report, indent=2)+"\n")
    print(json.dumps({k: v for k, v in report.items() if k not in ["query_ids", "statement_counts"]}))
    return 0 if report["verified"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
