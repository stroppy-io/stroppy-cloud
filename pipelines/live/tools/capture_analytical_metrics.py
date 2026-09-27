#!/usr/bin/env python3
"""Capture native query counts and database exporter evidence via the server."""
import argparse
import json
from pathlib import Path

from server_api import Client


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--tenant", required=True)
    p.add_argument("--run-id", required=True)
    p.add_argument("--evidence-dir", required=True)
    p.add_argument("--database-prefix", choices=["pg", "mysql"], default="pg")
    p.add_argument("--database-end-workload", action="store_true", help="Check DB health before deliberate teardown stops its container.")
    a = p.parse_args()
    c = Client()
    base = f"/api/v1/t/{a.tenant}/runs/{a.run_id}"
    r = c.request(base)
    assert r["http_status"] == 200
    run = r["body"]
    assert run["status"] == "completed"
    dest = Path(a.evidence_dir)
    dest.mkdir(parents=True, exist_ok=True)
    interval = {"start": run["started_at"], "end": run["finished_at"], "step": "5s"}
    queries = c.request(base + "/metrics:raw", "POST", {
        **interval, "query": "stroppy_run_query_operations_total",
    }, evidence=dest/"query-metrics.json")
    assert queries["http_status"] == 200
    db_request = {**interval, "query": '{__name__=~"' + a.database_prefix + '_.*"}'}
    if a.database_end_workload:
        db_request["end"] = run["result"]["segments"][-1]["finished_at"]
    db = c.request(base + "/metrics:raw", "POST", db_request)
    assert db["http_status"] == 200
    series = db["body"]["data"]["result"]
    summary = {"run_id": a.run_id, "http_status": db["http_status"], "query": db_request, "series_count": len(series),
               "series": [{"metric": s["metric"], "points": len(s.get("values", [])),
                           "last": s["values"][-1] if s.get("values") else None}
                          for s in series]}
    (dest/"database-metrics.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps({"query_series": len(queries["body"]["data"]["result"]), "database_series": len(series)}))


if __name__ == "__main__":
    main()
