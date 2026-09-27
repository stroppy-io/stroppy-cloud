#!/usr/bin/env python3
"""Record verified successes and blockers of the fixed 2026-09-27 campaign."""
from datetime import datetime, timezone
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PLATFORM = ROOT/"tests/platform/server/resilience"


def load(p):
    return json.loads(p.read_text())


def update(case, expected_run, changes):
    report = load(case/"checks.json")
    assert report["run_id"] == expected_run
    for check in report["checks"]:
        if check["id"] in changes:
            status, reason, path, pointer = changes[check["id"]]
            check.update(status=status, reason=reason,
                         evidence={"path": str(path.relative_to(ROOT)), "pointer": pointer} if path else None)
    report["checked_at"] = datetime.now(timezone.utc).isoformat()
    (case/"checks.json").write_text(json.dumps(report, indent=2)+"\n")


def main():
    for key, rid, path, campaign in [
        ("galera", "5fc7ddd6-5c13-4be7-b51b-21bf19277540", "mariadb/11.8/galera-proxysql/execute-sql/server-3nodes-2m", "ha-20260927/mariadb"),
        ("generated", "7b3d8ae2-1b43-4b41-81ca-64f9e522ea86", "postgres/17/single/tpcds/server-sf001-generated", "analytical-generated-20260927"),
    ]:
        case, evidence = ROOT/"tests"/path, PLATFORM/campaign
        run_path = case/"runs"/rid/"run.json"
        run = load(run_path)["body"]
        assert run["status"] == "completed"
        segment = run["result"]["segments"][0]
        assert segment["status"] == "completed" and segment["metrics"]["failed_queries_total"]["value"] == 0
        telemetry = load(evidence/"telemetry.json")
        assert telemetry["native_verified"] and telemetry["logs_verified"] and telemetry["invalid_metric_values"]
        for name in ["cleanup", "artifacts"]:
            proof = load(evidence/(name+".json"))
            assert proof["run_id"] == rid and proof["verified"]
        changes = {
            "infrastructure_provisioning": ("passed", "Required resources/agents became ready and executed the workload; exact hardware SKU not asserted.", run_path, "/body/result"),
            "native_tps": ("not_applicable", "This test reports native queries/s and iterations/s, not logical transaction TPS.", None, None),
            "native_otlp_metrics": ("passed", "Native workload telemetry is available with correct run/namespace scope.", evidence/"telemetry.json", "/native_verified"),
            "component_logs": ("passed", "Component logs and facets are available through the server.", evidence/"telemetry.json", "/logs_verified"),
            "component_metrics": ("failed", "Host CPU contains impossible negative percentages near exporter shutdown; Graphene brief records the raw-series reproduction.", evidence/"telemetry.json", "/invalid_metric_values"),
            "artifacts": ("passed", "All config/log artifacts downloaded after completion; sizes and SHA-256 match.", evidence/"artifacts.json", "/verified"),
            "cleanup": ("passed", "Completed run has no remaining infrastructure or kept stand.", evidence/"cleanup.json", "/verified"),
            "managed_database_metrics": ("not_applicable", "Self-managed database.", None, None),
        }
        if key == "galera":
            assert segment["metrics"]["run_query_operations_total"]["value"] == 20550
            metrics = load(evidence/"database-metrics.json")
            checks = {}
            for metric, want in [("mysql_up", 1), ("mysql_global_status_wsrep_cluster_size", 3), ("mysql_global_status_wsrep_local_state", 4),
                                 ("mysql_global_status_wsrep_ready", 1), ("mysql_global_status_wsrep_connected", 1)]:
                found = {s["metric"]["graphene.agent"]: float(s["last"][1]) for s in metrics["series"] if s["metric"]["__name__"] == metric}
                assert len(found) == 3 and set(found.values()) == {want}
                checks[metric] = found
            (evidence/"cluster-health.json").write_text(json.dumps({"run_id": rid, "verified": True, "member_metrics": checks,
                "sql_calls": 20550, "failed_queries": 0, "scope": "Health of all three members and writes through ProxySQL; no failover or per-replica row comparison."}, indent=2)+"\n")
            changes["smoke"] = ("passed", "20550 successful cluster-membership assertions and writes through ProxySQL over two minutes; all three members healthy.", evidence/"cluster-health.json", "/verified")
            changes["replication"] = ("not_run", "Three synced members and successful writes are verified; per-replica row comparison and failover remain untested.", evidence/"cluster-health.json", "/scope")
        else:
            coverage = load(evidence/"query-coverage.json")
            assert coverage["run_id"] == rid and coverage["missing_queries"] == [88]
            changes["smoke"] = ("failed", "Generated stream executes 102 statements / 98 of 99 queries without SQL errors; Q88 is silently skipped by Stroppy.", evidence/"query-coverage.json", "/missing_queries")
            changes["replication"] = ("not_applicable", "Single PostgreSQL instance.", None, None)
        update(case, rid, changes)
    rid = "22a00ddf-a390-4e89-b04a-a2eac80894bc"
    case = ROOT/"tests/mysql/8.4/group-proxysql/execute-sql/server-3nodes-2m"
    proof = PLATFORM/"graphene-0.2.26-blockers/cleanup-verification.json"
    snapshot = load(proof)
    assert snapshot["run_id"] == rid and snapshot["all_resources_deleted"] and snapshot["server_status"] == "cancelling"
    changes = {"cleanup": ("failed", "All 16 resources are deleted, but Graphene repeatedly retries server.run.cleanup and cancellation never reaches terminal state.", proof, "/cleanup_retries"),
               "infrastructure_provisioning": ("passed", "All five machines and agents became ready before image-pull failures.", case/"runs"/rid/"events.json", "/body/data")}
    for name in ["smoke", "native_otlp_metrics", "artifacts"]:
        changes[name] = ("blocked", "Cancelled before workload because public registry pulls repeatedly timed out.", PLATFORM/"ha-20260927/mysql/registry-errors.json", "/body/data")
    update(case, rid, changes)


if __name__ == "__main__": main()
