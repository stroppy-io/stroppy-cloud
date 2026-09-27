#!/usr/bin/env python3
"""Verify run telemetry through Stroppy Cloud only; save counts, not log bodies."""

import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
from urllib.parse import urlencode

from server_api import Client


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tenant", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--evidence", required=True)
    args = parser.parse_args()
    client = Client()
    base = f"/api/v1/t/{args.tenant}/runs/{args.run_id}"
    run = client.request(base)
    if run["http_status"] != 200:
        raise RuntimeError(f"run HTTP {run['http_status']}")
    body = run["body"]
    start = body.get("started_at") or body["created_at"]
    end = body.get("finished_at") or datetime.now(timezone.utc).isoformat()
    metrics = client.request(base + "/metrics?keys=iterations_total,iterations_per_second,queries_per_second,node_cpu_usage,node_memory_used_bytes")
    raw = client.request(base + "/metrics:raw", "POST", {
        "query": '{__name__=~"stroppy_.*"}', "start": start, "end": end, "step": "5s",
    })
    logs = client.request(base + "/logs?limit=20")
    facets = client.request(base + "/logs:facets")
    pages = [logs]
    older = logs["body"].get("older") if logs["http_status"] == 200 else None
    if older:
        pages.append(client.request(base + "/logs?" + urlencode({"limit": 20, "cursor": older, "direction": "older"})))
    forward = [client.request(base + "/logs?direction=newer&limit=20")]
    newer = forward[0]["body"].get("newer") if forward[0]["http_status"] == 200 else None
    if newer:
        forward.append(client.request(base + "/logs?" + urlencode({"limit": 20, "cursor": newer, "direction": "newer"})))
    raw_series = raw["body"].get("data", {}).get("result", []) if raw["http_status"] == 200 else []
    series = metrics["body"].get("series", []) if metrics["http_status"] == 200 else []
    invalid_values = []
    for s in series:
        if s.get("key") == "node_cpu_usage":
            aggregates = s.get("aggregates") or {}
            # A one percentage-point tolerance permits small counter clock
            # jitter; values such as -100% are a telemetry failure, not proof
            # of acceptance just because points were returned.
            if aggregates.get("min", 0) < -1 or aggregates.get("max", 100) > 101:
                invalid_values.append({"key": s["key"], "machine": s.get("machine"), "aggregates": aggregates})
    identity = body.get("graphene", {})
    expected_ns = identity.get("namespace", "t-" + args.tenant)
    expected_run = identity.get("run_ref", "run/" + args.run_id).removeprefix("run/")
    wrong_scope = []
    for s in raw_series:
        labels = s.get("metric", {})
        ns = labels.get("graphene_namespace", labels.get("graphene.namespace"))
        rid = labels.get("graphene_run", labels.get("graphene.run"))
        if ns != expected_ns or rid != expected_run:
            wrong_scope.append(labels)
    record = {
        "observed_at": datetime.now(timezone.utc).isoformat(), "run_id": args.run_id,
        "run_status": body["status"], "contract_version": run["contract_version"],
        "http": {"metrics": metrics["http_status"], "raw_metrics": raw["http_status"],
                 "logs": [p["http_status"] for p in pages],
                 "forward_logs": [p["http_status"] for p in forward], "facets": facets["http_status"]},
        "metric_errors": metrics["body"].get("errors", []),
        "metric_series": [{"key": s["key"], "points": len(s.get("points", [])),
                           "machine": s.get("machine"), "aggregates": s.get("aggregates")} for s in series],
        "native_series_count": len(raw_series),
        "native_metric_names": sorted({s.get("metric", {}).get("__name__", "") for s in raw_series}),
        "native_scope_mismatches": wrong_scope,
        "invalid_metric_values": invalid_values,
        "log_page_sizes": [len(p["body"].get("data", [])) for p in pages],
        "forward_log_page_sizes": [len(p["body"].get("data", [])) for p in forward],
        "facets": facets["body"].get("data", []),
    }
    record["logs_verified"] = (
        all(p["http_status"] == 200 and p["body"].get("data") for p in pages)
        and all(p["http_status"] == 200 for p in forward)
        and bool(newer)
        and facets["http_status"] == 200
    )
    record["native_verified"] = (
        metrics["http_status"] == raw["http_status"] == 200
        and len(raw_series) > 0 and not wrong_scope and not record["metric_errors"]
        and any(s["key"] == "iterations_total" and s.get("points") for s in series)
    )
    record["components_verified"] = (
        metrics["http_status"] == 200 and not record["metric_errors"] and not invalid_values
        and any(s["key"].startswith("node_") and s.get("points") for s in series)
    )
    record["verified"] = record["logs_verified"] and record["native_verified"] and record["components_verified"]
    dest = Path(args.evidence)
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps(record, indent=2))
    return 0 if record["verified"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
