#!/usr/bin/env python3
"""Qualify a completed analytical case using server-captured evidence only."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load(path):
    return json.loads(path.read_text())


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--case", required=True)
    p.add_argument("--campaign", required=True)
    a = p.parse_args()
    case = ROOT/"tests"/a.case
    evidence = ROOT/"tests/platform/server/resilience"/a.campaign
    checks = load(case/"checks.json")
    run_dir = case/"runs"/checks["run_id"]
    run = load(run_dir/"run.json")["body"]
    assert run["status"] == "completed"
    segments = run["result"]["segments"]
    assert segments and all(s["status"] == "completed" and s["exit_code"] == 0 and
                            s["metrics"]["failed_queries_total"]["value"] == 0 and
                            s["metrics"]["failed_iterations_total"]["value"] == 0 for s in segments)
    for name in ["cleanup", "artifacts"]:
        proof = load(evidence/(name+".json"))
        assert proof["verified"] and proof["run_id"] == run["id"]
    telemetry = load(evidence/"telemetry.json")
    assert telemetry["run_id"] == run["id"] and telemetry["native_verified"] and telemetry["logs_verified"]
    query_metrics = load(evidence/"query-metrics.json")
    assert query_metrics["http_status"] == 200
    counts = {}
    for s in query_metrics["body"]["data"]["result"]:
        labels = s["metric"]
        if labels.get("step") == "workload":
            assert labels.get("graphene.run") == run["id"]
            counts[labels["stroppy.segment"]] = float(s["values"][-1][1])
    expected = {s["name"]: 22 if s["name"] == "tpch" else 103 for s in segments}
    assert counts == expected, (counts, expected)
    db_metrics = load(evidence/"database-metrics.json")
    assert db_metrics["run_id"] == run["id"] and db_metrics["http_status"] == 200 and db_metrics["series_count"] > 0
    summary = {"verified": telemetry["verified"], "workload_verified": True, "run_id": run["id"], "native_workload_query_counts": counts,
               "segments": [{"name": s["name"], "status": s["status"], "failed_queries": s["metrics"]["failed_queries_total"]["value"],
                             "native_queries_per_second": s["metrics"]["queries_per_second"]} for s in segments],
               "note": "Small-scale functional acceptance; not TPC benchmark certification. Throughput copied from Stroppy, never calculated here."}
    (evidence/"verification.json").write_text(json.dumps(summary, indent=2)+"\n")
    updates = {
        "smoke": ("passed", "All analytical segments completed without query/iteration failures; native workload counters match the full query set.", evidence/"verification.json", "/workload_verified"),
        "infrastructure_provisioning": ("passed", "Successful workload on both prepared hosts. Exact CPU and disk SKU conformance is not asserted.", run_dir/"run.json", "/body/result"),
        "native_tps": ("not_applicable", "Analytical workload reports queries/s and iterations/s rather than logical transaction TPS.", None, None),
        "native_otlp_metrics": ("passed", "Native Stroppy metrics retrieved through the server with correct run/namespace scope.", evidence/"telemetry.json", "/native_verified"),
        "component_metrics": ("passed", "Host CPU/memory and PostgreSQL exporter series retrieved through the server.", evidence/"database-metrics.json", "/series_count"),
        "component_logs": ("passed", "Logs, pagination and facets include the database and exporters.", evidence/"telemetry.json", "/facets"),
        "artifacts": ("passed", "All configs/logs downloaded after completion and matched their sizes and SHA-256.", evidence/"artifacts.json", "/verified"),
        "cleanup": ("passed", "No infrastructure remains in the server resource tree.", evidence/"cleanup.json", "/verified"),
        "replication": ("not_applicable", "Single PostgreSQL instance.", None, None),
        "managed_database_metrics": ("not_applicable", "Self-managed PostgreSQL.", None, None),
    }
    if not telemetry["components_verified"]:
        updates["component_metrics"] = ("failed", "Full-run verification found impossible negative CPU values near exporter shutdown; see Graphene brief.", evidence/"telemetry.json", "/invalid_metric_values")
    for check in checks["checks"]:
        if check["id"] in updates:
            status, reason, path, pointer = updates[check["id"]]
            check.update(status=status, reason=reason, evidence={"path": str(path.relative_to(ROOT)), "pointer": pointer} if path else None)
    checks["checked_at"] = datetime.now(timezone.utc).isoformat()
    (case/"checks.json").write_text(json.dumps(checks, indent=2)+"\n")
    print(json.dumps(summary))


if __name__ == "__main__": main()
