#!/usr/bin/env python3
"""Check host CPU through the server, including the interval after teardown."""
import argparse
from datetime import datetime, timedelta, timezone
import json
import math
from pathlib import Path

from server_api import Client


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--tenant", required=True)
    p.add_argument("--run-id", required=True)
    p.add_argument("--evidence", required=True)
    a = p.parse_args()
    c = Client()
    base = f"/api/v1/t/{a.tenant}/runs/{a.run_id}"
    r = c.request(base)
    assert r["http_status"] == 200, r["http_status"]
    run = r["body"]
    assert run.get("finished_at"), "wait for terminal run before checking the tail"
    finished = datetime.fromisoformat(run["finished_at"].replace("Z", "+00:00"))
    end = finished + timedelta(minutes=2)
    assert datetime.now(timezone.utc) >= end, "wait two minutes after completion for the full tail"
    query = '100 * (1 - avg by ("graphene.agent") (rate(node_cpu_seconds_total{mode="idle"}[1m])))'
    request = {"query": query, "start": run["started_at"], "end": end.isoformat(), "step": "5s"}
    dest = Path(a.evidence)
    dest.parent.mkdir(parents=True, exist_ok=True)
    raw = c.request(base + "/metrics:raw", "POST", request,
                    evidence=dest.with_name(dest.stem + "-raw.json"))
    series = raw["body"].get("data", {}).get("result", []) if raw["http_status"] == 200 else []
    summaries = []
    for s in series:
        values = [float(v) for _, v in s.get("values", [])]
        finite = [v for v in values if math.isfinite(v)]
        summaries.append({"agent": s["metric"].get("graphene.agent"), "points": len(values),
                          "min": min(finite) if finite else None, "max": max(finite) if finite else None,
                          "verified": bool(finite) and len(finite) == len(values) and min(finite) >= -1 and max(finite) <= 101})
    expected = {a.run_id[:8] + "-" + m["name"] for m in run["run_spec"]["values"]["machines"]}
    observed = {s["agent"] for s in summaries}
    report = {"observed_at": datetime.now(timezone.utc).isoformat(), "run_id": a.run_id,
              "query": request, "http_status": raw["http_status"], "series": summaries,
              "expected_agents": sorted(expected), "observed_agents": sorted(observed),
              "verified": raw["http_status"] == 200 and observed == expected and bool(summaries)
                          and all(s["verified"] for s in summaries)}
    dest.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))
    return 0 if report["verified"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
