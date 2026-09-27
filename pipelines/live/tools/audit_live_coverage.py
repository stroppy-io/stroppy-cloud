#!/usr/bin/env python3
"""Audit retained evidence; optionally resolve provisioning unknowns without launches."""
import argparse
from collections import Counter, defaultdict
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path

import live


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def provisioning(root, case, folder):
    run_dir = folder / "runs" / case["selected_run"]
    inp_path, metrics_path = run_dir / "input.json", run_dir / "metrics.json"
    # Native-workload counters and per-host evidence are separate in these
    # retained runs. Never treat native metrics as a failed host observation.
    components_path = run_dir / "components.json"
    if components_path.is_file():
        components = live.read(components_path)
        if components.get("exporter_agents_observed"):
            metrics_path = components_path
    if not inp_path.is_file() or not metrics_path.is_file():
        return {"status": "unknown", "reason": "Retained input or per-host metrics missing."}
    inp, metrics = live.read(inp_path), live.read(metrics_path)
    sources = [{"path": str(p.relative_to(root)), "sha256": sha(p)} for p in (inp_path, metrics_path)]
    resolution = "retained per-run input"
    # The first migration kept PG templates as input.json. Use the saved
    # executed suite cell only when BOTH its durable run name and all later
    # per-host observations agree. Preserve the original snapshot unchanged.
    run_id = case["selected_run"]
    for prefix, filename in (("versions", "suite-postgres-versions.json"),
                             ("retry", "suite-postgres-versions-retry.json"),
                             ("final", "suite-postgres-versions-final.json")):
        suite_path = root / "tests/platform/suites" / filename
        if not run_id.startswith("matrix-pg-"+prefix+"-") or not suite_path.is_file():
            continue
        suite = live.read(suite_path)
        for index, cell in enumerate(suite["cells"]):
            expected = f"matrix-pg-{prefix}-{suite['suite_run_id'][:8]}-{cell['id']}"
            if expected == run_id:
                inp = cell["run_spec"]
                resolution = "executed suite cell; original per-run snapshot is a pre-launch template"
                sources.append({"path": str(suite_path.relative_to(root)), "pointer": f"/cells/{index}/run_spec", "sha256": sha(suite_path)})
    missing = []
    identity = inp.get("run_id", "")
    run_id = case["selected_run"]
    if not identity or metrics.get("run_id") != run_id or run_id not in metrics.get("exporter_runs_observed", []):
        missing.append("input/metrics execution identity not proven")
    machines = inp.get("machines", [])
    if not machines:
        missing.append("no machine inventory")
    checked = []
    for machine in machines:
        name = machine["name"]
        entity = f"docker/{identity}-{name}-node-exporter"
        agent = f"{identity[:8]}-{name}"
        if agent not in metrics.get("exporter_agents_observed", []):
            missing.append(f"agent {agent}")
        if metrics.get("exporters", {}).get(entity, {}).get("metric_names", 0) <= 0:
            missing.append(f"host metrics {entity}")
        for disk in machine.get("disks", []):
            observed = [d for d in metrics.get("disk_info", []) if d.get("entity") == entity and d.get("serial") == disk["name"]]
            if not observed:
                missing.append(f"disk {name}/{disk['name']}")
            if disk.get("mount"):
                devices = {"/dev/" + d["device"] for d in observed}
                filesystems = [d for d in metrics.get("data_filesystems", []) if d.get("entity") == entity and d.get("device") in devices]
                # Filesystem metadata consumes capacity; this confirms a data
                # filesystem of the expected order of size, not exact YC SKU.
                if not any(d.get("size_bytes", 0) >= disk["gb"] * 2**30 * 0.9 for d in filesystems):
                    missing.append(f"data filesystem {name}/{disk['name']}")
        checked.append({"machine": name, "agent": agent, "entity": entity, "disks": [d["name"] for d in machine.get("disks", [])]})
    if inp.get("managed_ydb"):
        missing.append("managed YDB resource readiness requires separate evidence")
    return {
        "status": "unknown" if missing else "passed",
        "run_id": run_id,
        "input_run_id": identity,
        "input_resolution": resolution,
        "reason": "Missing observations: " + "; ".join(missing) if missing else "Retained run-scoped telemetry identifies every expected agent, host exporter and data disk; mounted disks also have a data filesystem. Historical provisioning evidence, not a new server/API run or exact cloud hardware conformance.",
        "checked_machines": checked,
        "sources": sources,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="Update only proven provisioning checks and rebuild the one CSV")
    args = parser.parse_args()
    root = live.ROOT
    live.validate(root)
    cases, unknown, resolved, remaining = [], Counter(), [], []
    mechanisms = defaultdict(set)
    run_workloads = set()
    run_reports = {}
    now = datetime.now(timezone.utc).isoformat()
    for path, case, record in live.cases(root):
        if case.get("provider") == "aws":
            continue
        for check in record["checks"]:
            if check["status"] == "unknown":
                unknown[check["id"]] += 1
            previous_audit = (check.get("evidence") or {}).get("path", "").endswith("/infrastructure-audit.json")
            if check["id"] != "infrastructure_provisioning" or (check["status"] != "unknown" and not previous_audit) or not case.get("selected_run"):
                continue
            proof = provisioning(root, case, path.parent)
            report_path = path.parent / "runs" / case["selected_run"] / "infrastructure-audit.json"
            run_reports[report_path.resolve()] = (report_path, proof)
            if proof["status"] == "passed":
                resolved.append(case["id"])
            else:
                remaining.append({"case": case["id"], "reason": proof["reason"]})
            if args.apply:
                check.update(status=proof["status"], reason=proof["reason"], evidence={"path": str(report_path.relative_to(root)), "pointer": "/status"}, audited_at=now)
                (path.parent / case["checks"]).write_text(json.dumps(record, indent=2, ensure_ascii=False) + "\n")
        if case["execution"]["status"] == "completed":
            mechanisms[f"{case['database']}/{case['version']}/{case['topology']}"].add(case["selected_run"])
            for segment in case.get("actual_segments", []):
                script = (segment.get("script") or segment.get("params", {}).get("script")
                          or segment.get("workload", {}).get("script"))
                if script:
                    run_workloads.add((case["selected_run"], script))
        cases.append(case["id"])
    if args.apply:
        for path, proof in run_reports.values():
            path.write_text(json.dumps({"audited_at": now, **proof}, indent=2, ensure_ascii=False) + "\n")
        live.progress(root)
    workloads = Counter(script for _, script in run_workloads)
    result = {
        "audited_at": now, "applied": args.apply, "cases": len(cases),
        "unknown_before": dict(unknown), "provisioning_proven_cases": len(resolved),
        "provisioning_proven_unique_runs": len({proof["run_id"] for _, proof in run_reports.values() if proof["status"] == "passed"}),
        "resolved_cases": resolved, "remaining_unknown": remaining,
        "workloads_in_completed_run_inputs": dict(workloads),
        "historical_topologies": {k: sorted(v) for k, v in sorted(mechanisms.items())},
        "missing_analytical_evidence": [s for s in ("tpch/tx", "tpcds") if s not in workloads],
        "limits": "Workload inventory is input evidence, not a new execution assertion. Historical topology evidence does not prove failover under faults. All new runs must use server API; AWS excluded. The project still has one progress.csv.",
    }
    dest = root / "tests/platform/server/resilience/coverage-audit.json"
    dest.write_text(json.dumps(result, indent=2, ensure_ascii=False) + "\n")
    print(json.dumps({k: result[k] for k in ("cases", "unknown_before", "provisioning_proven_cases", "provisioning_proven_unique_runs", "missing_analytical_evidence")}))


if __name__ == "__main__":
    main()
