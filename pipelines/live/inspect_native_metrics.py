#!/usr/bin/env python3
"""Compare Stroppy's own final measurements with durable Graphene metric reads."""
import argparse
import base64
import datetime as dt
import json
import math
from pathlib import Path
import subprocess
import urllib.parse
import urllib.request

NAMESPACE = "t-stroppy-live"
NAMES = {
    "tps": "stroppy_tps_per_second",
    "iterations_per_second": "stroppy_iterations_per_second_per_second",
    "queries_per_second": "stroppy_queries_per_second_per_second",
    "measurement_seconds": "stroppy_measurement_seconds",
    "successful_transactions_total": "stroppy_successful_transactions_total",
    "iterations_total": "stroppy_iterations_total",
    "failed_iterations_total": "stroppy_failed_iterations_total",
    "failed_queries_total": "stroppy_failed_queries_total",
    "terminal_errors_total": "stroppy_terminal_errors_total",
    "retry_attempts_total": "stroppy_retry_attempts_total",
}

def parse_time(value):
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))

def inspect(cli, run_id, result, *, require_zero_errors=True, endpoint=None):
    checks = []
    instances = set()
    for segment in result["segments"]:
        if segment["status"] != "completed" or segment.get("exit_code", 0):
            raise ValueError("segment did not complete: " + segment["name"])
        # Compare final native measurements, retained by the backend after
        # process exit. A whole-segment query also includes every DB exporter
        # and can exceed Graphene's response limit on larger clusters.
        start = parse_time(segment["finished_at"])
        end = start + dt.timedelta(seconds=30)
        command = [cli, "-n", NAMESPACE, "metrics", "run", run_id,
                   "--start", start.isoformat(), "--end", end.isoformat(), "--jq", "."]
        if endpoint:
            selector = "{" + ",".join(key + "=" + json.dumps(value) for key, value in {
                "graphene_run": run_id, "graphene_namespace": NAMESPACE,
                "stroppy_segment": segment["name"],
            }.items()) + "}"
            lower = parse_time(segment["started_at"]).timestamp() * 1000
            upper = end.timestamp() * 1000
            query = urllib.parse.urlencode({"match[]": selector, "start": segment["started_at"], "end": end.isoformat()})
            merged = {}
            with urllib.request.urlopen(endpoint.rstrip("/") + "/api/v1/export?" + query, timeout=45) as response:
                for line in response:
                    if not line.strip():
                        continue
                    row = json.loads(line)
                    assert row["metric"].get("graphene_run") == run_id
                    assert row["metric"].get("graphene_namespace") == NAMESPACE
                    assert row["metric"].get("stroppy_segment") == segment["name"]
                    assert len(row["timestamps"]) == len(row["values"])
                    key = tuple(sorted(row["metric"].items()))
                    target = merged.setdefault(key, {"metric": row["metric"], "values": []})
                    target["values"].extend((stamp / 1000, value) for stamp, value in zip(row["timestamps"], row["values"])
                                            if lower <= stamp <= upper)
            rows = list(merged.values())
            for row in rows:
                row["values"] = sorted(set(row["values"]))
            rows = [row for row in rows if row["values"]]
            query_start = segment["started_at"]
            source = "persistent VictoriaMetrics raw export; exact native namespace/run/segment labels"
        else:
            response = json.loads(subprocess.check_output(command, text=True, timeout=45))
            snapshot = json.loads(base64.b64decode(response["snapshot"]))
            if snapshot["status"] != "success":
                raise ValueError("backend query failed")
            rows = [r for r in snapshot["data"]["result"]
                    if r["metric"].get("stroppy_segment") == segment["name"]
                    and r["metric"].get("graphene_run") == run_id
                    and r["metric"].get("graphene_namespace") == NAMESPACE
                    and r.get("values")]
            query_start = start.isoformat()
            source = "authenticated Graphene Metrics API / persistent VictoriaMetrics"
        if not rows:
            raise ValueError("no persisted native metrics for " + segment["name"])
        writer_ids = {r["metric"].get("instance", "") for r in rows}
        if len(writer_ids) != 1 or "" in writer_ids or instances & writer_ids:
            raise ValueError("missing or reused native metric writer identity")
        instances.update(writer_ids)
        summary = segment["metrics"]
        values = {}
        for name, exported in NAMES.items():
            matching = [r for r in rows if r["metric"]["__name__"] == exported]
            if name not in summary:
                if name == "tps" and matching:
                    raise ValueError("unexpected TPS for nontransactional workload")
                continue
            if not matching:
                raise ValueError("missing exported " + exported)
            value = sum(float(r["values"][-1][1]) for r in matching)
            if not math.isfinite(value) or abs(value - summary[name]["value"]) > 0.00051:
                raise ValueError(f"{segment['name']} {name}: backend {value} != native summary {summary[name]['value']}")
            values[name] = value
        for name in ("failed_iterations_total", "failed_queries_total", "terminal_errors_total"):
            if name not in values:
                raise ValueError("missing explicit error counter: " + name)
        # Failed query attempts may be recovered by a transaction retry. They
        # remain visible even when every logical transaction eventually succeeds.
        if require_zero_errors:
            for name in ("failed_iterations_total", "terminal_errors_total"):
                if values[name] != 0:
                    raise ValueError("nonzero terminal error counter: " + name)
        if values.get("iterations_total", 0) <= 0:
            raise ValueError("no completed iterations")
        checks.append({"segment": segment["name"], "run_id": run_id,
                       "source": source,
                       "series": len(rows), "instance": next(iter(writer_ids)),
                       "metric_names": sorted({r["metric"]["__name__"] for r in rows}),
                       "measurements": values, "native_summary": summary,
                       "workload_without_terminal_errors": values["terminal_errors_total"] == 0
                           and values["failed_iterations_total"] == 0,
                       "compliance": segment.get("compliance"),
                       "query_window": {"start": query_start, "end": end.isoformat()},
                       "query_scope": "last original sample per native series in the workload window" if endpoint else "final native measurements after segment exit; not a full time-series export",
                       "status": "passed"})
    return {"status": "passed", "run_id": run_id, "segments": checks}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("run_id")
    parser.add_argument("result", type=Path)
    parser.add_argument("--cli", default="graphenectl")
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--allow-workload-errors", action="store_true",
                        help="verify export fidelity while retaining nonzero workload error counters")
    args = parser.parse_args()
    report = inspect(args.cli, args.run_id, json.loads(args.result.read_text()),
                     require_zero_errors=not args.allow_workload_errors)
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    print(f"{args.run_id}: {len(report['segments'])} segments verified in persistent storage")

if __name__ == "__main__":
    main()
