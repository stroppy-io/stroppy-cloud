#!/usr/bin/env python3
"""Wait for the three recorded HA runs, then qualify them through server APIs only."""
from datetime import datetime, timezone
import argparse
import json
from pathlib import Path
import subprocess
import sys
import time

from record_20260927_checks import ROOT, load, update
from server_api import Client

CASES = {"mysql": "mysql/8.4/group-proxysql/failover/server-primary-rejoin",
         "mariadb": "mariadb/11.8/galera-proxysql/failover/server-primary-rejoin",
         "postgres": "postgres/17/patroni-ha/failover/server-primary-rejoin"}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--campaign", required=True)
    p.add_argument("--tenant", default="server-live-20260924")
    p.add_argument("--kind", choices=list(CASES), action="append")
    a = p.parse_args()
    campaign = ROOT/"tests/platform/server/resilience"/a.campaign
    client, pending, captured = Client(), {k: CASES[k] for k in (a.kind or CASES)}, set()
    started = time.monotonic()

    def tool(name, args, evidence):
        command = [sys.executable, str(ROOT/"tools"/(name+".py")), *args]
        result = subprocess.run(command, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        (evidence/(name+".log")).write_text(result.stdout)
        if result.returncode: raise RuntimeError(name+" failed; inspect "+str(evidence))
        print(name, "passed", evidence.name, flush=True)

    while pending and time.monotonic()-started < 3600:
        for kind, case_path in list(pending.items()):
            evidence = campaign/kind
            rid = load(evidence/"launch.json")["body"]["id"]
            response = client.request(f"/api/v1/t/{a.tenant}/runs/{rid}")
            if response["http_status"] != 200: continue
            run = response["body"]
            if run["status"] not in {"completed", "failed", "cancelled"}: continue
            if kind not in captured:
                (evidence/"final.json").write_text(json.dumps(response, indent=2)+"\n")
                common = ["--tenant", a.tenant, "--run-id", rid]
                tool("record_server_case", common+["--case", case_path, "--workload", "execute/sql", "--refresh"], evidence)
                tool("verify_server_cleanup", common+["--evidence", str(evidence/"cleanup.json")], evidence)
                assert run["status"] == "completed", f"{kind} run {run['status']}; inspect final.json"
                tool("verify_server_ha_failover", common+["--evidence-dir", str(evidence)], evidence)
                tool("verify_server_artifacts", common+["--evidence", str(evidence/"artifacts.json")], evidence)
                tool("verify_server_telemetry", common+["--evidence", str(evidence/"telemetry.json")], evidence)
                tool("capture_analytical_metrics", common+["--evidence-dir", str(evidence), "--database-prefix", "pg" if kind == "postgres" else "mysql", "--database-end-workload"], evidence)
                captured.add(kind)
            finished = datetime.fromisoformat(run["finished_at"].replace("Z", "+00:00"))
            if (datetime.now(timezone.utc)-finished).total_seconds() < 120: continue
            tool("verify_server_cpu_tail", ["--tenant", a.tenant, "--run-id", rid, "--evidence", str(evidence/"cpu-tail.json")], evidence)
            database = load(evidence/"database-metrics.json")
            name = "pg_up" if kind == "postgres" else "mysql_up"
            expected = {rid[:8]+"-"+c["machine"] for c in run["run_spec"]["values"]["containers"] if c["kind"] == "database"}
            found = {s["metric"]["graphene.agent"]: float(s["last"][1]) for s in database["series"] if s["metric"]["__name__"] == name and s["last"]}
            assert set(found) == expected and set(found.values()) == {1}, (kind, found, expected)
            proof = load(evidence/"ha-verification.json")
            assert proof["verified"] and proof["run_id"] == rid
            changes = {
                "smoke": ("passed", "Start and final SQL assertions passed; permitted errors during the controlled outage are retained in the result.", evidence/"ha-verification.json", "/verified"),
                "infrastructure_provisioning": ("passed", "All required machines executed the scenario. Exact hardware SKU is not asserted.", evidence/"final.json", "/body/result"),
                "replication": ("passed", proof["scope"], evidence/"ha-verification.json", "/verified"),
                "native_tps": ("not_applicable", "execute_sql reports native queries/s rather than logical transaction TPS.", None, None),
                "managed_database_metrics": ("not_applicable", "Self-managed database.", None, None),
            }
            for key, filename, pointer, reason in [
                ("native_otlp_metrics", "telemetry", "/native_verified", "Native Stroppy metrics retain correct run/namespace scope."),
                ("component_logs", "telemetry", "/logs_verified", "Component logs, facets and pagination are available through the server."),
                ("component_metrics", "cpu-tail", "/verified", "All host CPU series valid through teardown; all database exporters healthy after rejoin."),
                ("artifacts", "artifacts", "/verified", "Config/log artifacts match size and SHA after cleanup."),
                ("cleanup", "cleanup", "/verified", "No infrastructure or kept stand remains in the server resource tree."),
            ]: changes[key] = ("passed", reason, evidence/(filename+".json"), pointer)
            update(ROOT/"tests"/case_path, rid, changes)
            print(kind, "fully verified", rid, flush=True)
            pending.pop(kind)
        if pending: time.sleep(15)
    if pending: raise TimeoutError("still active; inspect the saved run IDs")
    tool("live", ["progress"], campaign)
    tool("live", ["validate"], campaign)


if __name__ == "__main__":
    main()
