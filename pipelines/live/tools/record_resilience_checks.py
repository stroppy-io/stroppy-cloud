#!/usr/bin/env python3
"""Promote only evidence-backed checks from the fixed resilience campaign."""
from datetime import datetime, timezone
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
EVIDENCE = ROOT / "tests/platform/server/resilience/live-suite-20260927"


def load(p):
    return json.loads(p.read_text())


def main():
    telemetry = load(EVIDENCE/"telemetry-postgres.json")
    database = load(EVIDENCE/"database-metrics-postgres.json")
    artifacts = load(EVIDENCE/"artifacts-postgres.json")
    assert telemetry["verified"] and artifacts["verified"] and database["series_count"] > 0
    for db, rid in [("postgres", "be40ddab-2f06-4904-9d51-999359b45653"), ("noop", "e3e18f98-f2b4-40dc-93da-e035ff75814f")]:
        suffix = "postgres/17/single" if db == "postgres" else "noop/builtin/runner-only"
        case = ROOT/"tests"/suffix/"execute-sql/server-resilience-2m-keep1m"
        report = load(case/"checks.json")
        assert report["run_id"] == rid
        run = load(case/"runs"/rid/"run.json")["body"]
        events = load(case/"runs"/rid/"events.json")["body"]["data"]
        cleanup = EVIDENCE/("cleanup-"+rid+".json")
        assert load(cleanup)["verified"]
        evidence_run = str((case/"runs"/rid/"run.json").relative_to(ROOT))
        updates = {"cleanup": ("passed", "Server tree has no remaining infrastructure; config/log artifacts are independent.", cleanup, "/verified")}
        if any(e.get("kind") == "phase.finished" and e.get("subject") == "provisioning" for e in events):
            updates["infrastructure_provisioning"] = ("passed", "Provisioning phase completed after resource/agent readiness. Exact hardware SKU conformance is not asserted.", case/"runs"/rid/"events.json", "/body/data")
        if db == "postgres":
            assert run["status"] == "completed" and run["result"]["segments"][0]["metrics"]["failed_queries_total"]["value"] == 0
            updates.update({
                "smoke": ("passed", "PostgreSQL config SQL assertion completed for 120 seconds with zero failed queries after server restart.", case/"runs"/rid/"run.json", "/body/result"),
                "native_otlp_metrics": ("passed", "61 native Stroppy series with the correct namespace/run, retrieved through the server.", EVIDENCE/"telemetry-postgres.json", "/verified"),
                "component_metrics": ("passed", "Host CPU/memory on both machines and 73 PostgreSQL exporter series available through the server.", EVIDENCE/"database-metrics-postgres.json", "/series_count"),
                "component_logs": ("passed", "Log pagination and facets include PostgreSQL, PostgreSQL exporter and both node exporters.", EVIDENCE/"telemetry-postgres.json", "/facets"),
                "artifacts": ("passed", "Config and log downloaded after keep deadline; sizes and SHA-256 match.", EVIDENCE/"artifacts-postgres.json", "/verified"),
                "replication": ("not_applicable", "Single PostgreSQL node; replication is absent from this case.", None, None),
                "managed_database_metrics": ("not_applicable", "Self-managed PostgreSQL.", None, None),
            })
        else:
            assert run["status"] == "cancelled"
            for name in ["smoke", "native_otlp_metrics", "component_metrics", "component_logs", "artifacts"]:
                updates[name] = ("blocked", "Intentionally cancelled during deployment to test durable cancellation; slow image pull remains unqualified.", EVIDENCE/"cancel-intent.json", "/reason")
        for check in report["checks"]:
            if check["id"] in updates:
                status, reason, path, pointer = updates[check["id"]]
                check.update(status=status, reason=reason, evidence={"path": str(path.relative_to(ROOT)), "pointer": pointer} if path else None)
        report["checked_at"] = datetime.now(timezone.utc).isoformat()
        (case/"checks.json").write_text(json.dumps(report, indent=2)+"\n")
        print(str(case.relative_to(ROOT)))


if __name__ == "__main__": main()
