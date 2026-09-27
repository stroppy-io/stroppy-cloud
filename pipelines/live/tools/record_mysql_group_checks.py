#!/usr/bin/env python3
"""Qualify the three-member Group Replication write check using server evidence."""
import argparse
import json
from pathlib import Path

from record_20260927_checks import ROOT, load, update


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--case", required=True)
    p.add_argument("--evidence-dir", required=True)
    a = p.parse_args()
    case, evidence = ROOT/"tests"/a.case, Path(a.evidence_dir).resolve()
    rid = load(case/"checks.json")["run_id"]
    run_path = case/"runs"/rid/"run.json"
    run = load(run_path)["body"]
    assert run["status"] == "completed"
    segments = run["result"]["segments"]
    assert len(segments) == 1 and segments[0]["name"] == "cluster-health-writes"
    segment = segments[0]
    assert segment["status"] == "completed" and segment["exit_code"] == 0
    metrics = segment["metrics"]
    assert metrics["failed_queries_total"]["value"] == metrics["failed_iterations_total"]["value"] == 0
    calls = metrics["run_query_operations_total"]["value"]
    assert calls > 0
    for f in ["cleanup", "artifacts", "telemetry", "cpu-tail"]:
        proof = load(evidence/(f+".json"))
        assert proof["run_id"] == rid and proof["verified"], f
    database = load(evidence/"database-metrics.json")
    assert database["run_id"] == rid and database["http_status"] == 200
    members = {s["metric"]["graphene.agent"]: float(s["last"][1])
               for s in database["series"] if s["metric"]["__name__"] == "mysql_up" and s["last"]}
    expected = {rid[:8]+"-"+name for name in ["db-1", "db-replica-1", "db-replica-2"]}
    assert set(members) == expected and set(members.values()) == {1}
    # Bind the result to the SQL assertion actually submitted, not merely its name.
    test = load(evidence/"test.json")["body"]
    init = test["database"]["inline"]["params"]["init_sql"]
    assert "COUNT(*)=3" in init and "MEMBER_STATE='ONLINE'" in init and "MEMBER_ROLE='PRIMARY'" in init
    assert "SIGNAL SQLSTATE '45000'" in init and "writes=writes+1" in init
    workload = run["run_spec"]["values"]["workload"]
    assert workload["segments"][0]["workload"]["sql_body"] == "CALL live_assert_cluster();"
    summary = {"run_id": rid, "verified": True, "successful_calls": calls,
               "failed_queries": 0, "mysql_up": members,
               "native_queries_per_second": metrics["queries_per_second"],
               "scope": "Three ONLINE members, one PRIMARY, writes via ProxySQL; per-replica row comparison and failover untested."}
    (evidence/"verification.json").write_text(json.dumps(summary, indent=2)+"\n")
    changes = {
        "smoke": ("passed", "Membership assertions and writes through ProxySQL succeeded without query errors.", evidence/"verification.json", "/verified"),
        "infrastructure_provisioning": ("passed", "Five machines deployed and workload completed; exact hardware SKU not asserted.", run_path, "/body/result"),
        "native_tps": ("not_applicable", "execute_sql reports queries/s, not logical transaction TPS.", None, None),
        "replication": ("not_run", summary["scope"], evidence/"verification.json", "/scope"),
        "managed_database_metrics": ("not_applicable", "Self-managed MySQL.", None, None),
    }
    for name, file, pointer, reason in [
        ("native_otlp_metrics", "telemetry", "/native_verified", "Native Stroppy metrics with correct run/namespace scope."),
        ("component_logs", "telemetry", "/logs_verified", "Component logs, facets and pagination available."),
        ("component_metrics", "cpu-tail", "/verified", "CPU valid for all five machines including shutdown; all three MySQL exporters healthy."),
        ("artifacts", "artifacts", "/verified", "Config/log downloads match SHA-256 and size after teardown."),
        ("cleanup", "cleanup", "/verified", "Terminal completed, no remaining infrastructure or kept stand."),
    ]:
        changes[name] = ("passed", reason, evidence/(file+".json"), pointer)
    update(case, rid, changes)
    print(json.dumps(summary))


if __name__ == "__main__":
    main()
