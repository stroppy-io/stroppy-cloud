#!/usr/bin/env python3
"""Record an API-launched attempt without promoting unverified workload checks."""

import argparse
import json
from pathlib import Path
import uuid

from live import REQUIRED_CHECKS, ROOT
from server_api import Client


def write(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tenant", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--case", required=True, help="database/version/topology/workload/preset under tests")
    parser.add_argument("--workload", required=True, help="display workload, e.g. execute/sql")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--append", action="store_true", help="append a new attempt, preserving the previous checks with its run")
    mode.add_argument("--refresh", action="store_true", help="refresh the selected attempt without changing acceptance checks")
    args = parser.parse_args()
    try:
        uuid.UUID(args.run_id)
    except ValueError:
        parser.error("server run id must be a UUID")
    parts = Path(args.case).parts
    if len(parts) != 5 or any(p in {"..", ".", "/"} for p in parts):
        parser.error("case must have five relative path components")
    dest = ROOT / "tests" / args.case
    previous = json.loads((dest / "case.json").read_text()) if (dest / "case.json").exists() else None
    if previous and not (args.append or args.refresh):
        parser.error("case exists; update its checks explicitly instead of discarding evidence")
    if args.refresh and (not previous or previous["selected_run"] != args.run_id):
        parser.error("refresh requires the selected run of an existing case")
    if previous and previous["workload"] != args.workload:
        parser.error("appended attempt must use the existing workload")
    run_dir = dest / "runs" / args.run_id
    run_dir.mkdir(parents=True, exist_ok=args.refresh)
    if previous and args.append:
        old_checks = dest / "runs" / previous["selected_run"] / "checks.json"
        if not old_checks.exists():
            old_checks.write_bytes((dest / "checks.json").read_bytes())
    client = Client()
    path = f"/api/v1/t/{args.tenant}/runs/{args.run_id}"
    observations = {}
    for suffix, name in [("", "run"), ("/tree", "tree"), ("/events", "events"), ("/artifacts", "artifacts")]:
        record = client.request(path + suffix)
        if record["http_status"] != 200:
            raise RuntimeError(f"{name}: HTTP {record['http_status']}")
        # RunSpec may carry operator OTLP credentials; never retain these in git.
        obs = record.get("body", {}).get("run_spec", {}).get("values", {}).get("observability", {})
        obs.pop("otlp_headers", None)
        obs.pop("headers", None)
        write(run_dir / (name + ".json"), record)
        observations[name] = record
    record = observations["run"]
    run = record["body"]
    spec = run["run_spec"]["values"]
    write(dest / "input.json", spec)
    write(run_dir / "input.json", spec)
    rel = str(dest.relative_to(ROOT))
    run_rel = str(run_dir.relative_to(ROOT))
    execution = {"status": run["status"], "observed_at": record["observed_at"]}
    evidence = {"path": run_rel + "/run.json", "pointer": "/body"}
    case = {"schema_version": 1, "id": rel, "database": parts[0], "version": parts[1],
            "topology": parts[2], "workload": args.workload, "preset": parts[4], "provider": "yandex",
            "zones": sorted({m["location"] for m in spec["machines"]}),
            "acceptance_scope": "real YC run launched and observed through the local server API",
            "selected_run": args.run_id, "execution": execution, "checks": "checks.json",
            "runs": previous["runs"] if args.refresh else (previous["runs"] if previous else []) + ["runs/" + args.run_id], "source": evidence,
            "input": "input.json", "input_status": "public_snapshot",
            "input_note": "Compiled RunSpec from the server API; operator OTLP headers omitted.",
            "actual_segments": spec["workload"]["segments"]}
    write(dest / "case.json", case)
    write(run_dir / "manifest.json", {"schema_version": 1, "run_id": args.run_id,
          "case_paths": [rel], "execution": execution, "input_status": "public_snapshot",
          "input_source": {"path": run_rel + "/run.json", "pointer": "/body/run_spec/values"}})
    if args.refresh:
        print(rel)
        return
    provisioning_failed = any(e.get("kind") == "phase.failed" and e.get("subject") == "provisioning"
                              for e in observations["events"]["body"]["data"])
    checks = []
    for name, group in REQUIRED_CHECKS.items():
        status, reason, proof = "not_run", "No explicit acceptance evidence recorded for this check.", None
        if name == "compile":
            status, reason, proof = "passed", "Server validated and compiled the test before launch.", evidence
        elif name == "infrastructure_provisioning" and provisioning_failed:
            status, reason, proof = "failed", run["status_reason"], evidence
        elif name in {"smoke", "native_otlp_metrics", "component_metrics", "component_logs", "artifacts"} and provisioning_failed:
            status, reason = "blocked", "Provisioning failed before workload execution."
        elif name == "native_tps" and args.workload in {"execute/sql", "simple"}:
            status, reason = "not_applicable", "The workload has no logical transaction TPS."
        elif name in {"replication", "database_metric_distributions", "managed_database_metrics"} and parts[0] == "noop":
            status, reason = "not_applicable", "Runner-only test has no database."
        checks.append({"id": name, "group": group, "status": status, "reason": reason, "evidence": proof})
    write(dest / "checks.json", {"schema_version": 1, "run_id": args.run_id,
          "checked_at": record["observed_at"], "checks": checks})
    print(rel)


if __name__ == "__main__":
    main()
