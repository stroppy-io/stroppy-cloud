#!/usr/bin/env python3
"""Rebuild the reviewable YC smoke matrix from saved verification evidence."""
import csv
import json
import os
import tempfile
from pathlib import Path

LIVE = Path(os.environ.get("STROPPY_LEGACY_LIVE", "/nonexistent/legacy-live-snapshot"))

def read(name):
    return json.loads((LIVE / name).read_text())

def current_run_stage(events):
    """Report the latest scheduled stage; this never asserts successful completion."""
    stages = {
        "k8s.entity.declare": "infrastructure",
        "docker.install": "docker_install",
        "stroppy.host.prep": "host_preparation",
        "stroppy.image.pull": "images",
        "docker.entity.declare": "containers",
        "stroppy.database.ready": "database_readiness",
        "stroppy.segment.run": "workload",
        "graphene.blob.upload-file": "artifacts",
        "server.artifact.declare": "artifacts",
        "server.run.cleanup": "cleanup",
    }
    for event in reversed(events):
        raw = event.get("raw", event)
        name = raw.get("activityTaskScheduledEventAttributes", {}).get("activityType", {}).get("name")
        if name in stages:
            return {"stage": stages[name], "activity": name, "scheduled_at": raw.get("eventTime", "")}
    return {}


def apply_current_campaigns(rows, snapshot):
    """Track the latest retry without replacing previously accepted evidence."""
    phases = snapshot.get("runs", {})
    key_fields = ("database", "version", "topology", "workload", "preset")
    indexed = {tuple(r[k] for k in key_fields): r for r in rows}
    latest = {}
    for planned in snapshot.get("planned_workloads", []):
        latest[tuple(planned[k] for k in key_fields[:4])] = planned
        key = tuple(planned[k] for k in key_fields)
        row = indexed.get(key)
        if row is not None and row["smoke"] == "passed":
            continue
        if row is not None and row.get("run_id") == planned["run_id"]:
            continue
        replacement = {k: "" for k in rows[0]}
        replacement.update(planned, provider="yandex", compile="passed", smoke="queued",
                           native_otlp_metrics="pending", component_metrics="pending",
                           component_logs="pending", artifacts="pending", pipeline_traces="pending",
                           cleanup="pending", evidence="current-campaigns.json",
                           note="Latest planned workload; workflow completion alone does not verify the result.")
        if row is None:
            rows.append(replacement)
            indexed[key] = replacement
        else:
            row.clear()
            row.update(replacement)
    for row in rows:
        # The catalogue smoke row summarizes the latest matching workload,
        # whereas native rows retain their own preset and accepted evidence.
        planned = latest.get(tuple(row[k] for k in key_fields[:4]))
        if row["smoke"] != "passed" and row["preset"].startswith("smoke-") and planned and row.get("run_id") != planned["run_id"]:
            for field in ("run_id", "input", "zone"):
                row[field] = planned[field]
            for field in ("component_metrics", "component_logs", "artifacts", "pipeline_traces", "native_otlp_metrics", "cleanup", "replication"):
                row[field] = "pending"
            for field in ("started_at", "finished_at", "iterations", "checked_at", "component_log_observations"):
                row[field] = ""
            row.update(smoke="queued", evidence="current-campaigns.json",
                       note="Latest retry; verification pending. Previous attempts remain in their evidence files.")
        recheck = planned if row["smoke"] == "passed" and planned and planned["run_id"] != row.get("run_id") else None
        row["recheck_run_id"] = recheck["run_id"] if recheck else ""
        row["recheck_phase"] = phases.get(recheck["run_id"], "Queued") if recheck else ""
        row["recheck_stage"] = snapshot.get("stages", {}).get(recheck["run_id"], {}).get("stage", "") if recheck else ""
        row["recheck_verification"] = snapshot.get("verification", {}).get(recheck["run_id"], "pending") if recheck else ""
        run_id = row.get("run_id", "")
        phase = phases.get(run_id, "")
        stage = snapshot.get("stages", {}).get(run_id, {})
        row["run_phase"] = phase
        row["run_status_checked_at"] = snapshot.get("checked_at", "") if phase else ""
        row["run_stage"] = stage.get("stage", "")
        row["run_stage_checked_at"] = stage.get("checked_at", "")
        if row["smoke"] != "passed" and phase:
            row["smoke"] = {"Running": "running", "Completed": "verification_pending",
                            "Failed": "failed", "Canceled": "canceled"}.get(phase, "queued")

def main():
    verified = {}
    for name in ("postgres-matrix-check.json", "patroni-matrix-check.json"):
        report = read(name)
        assert report["status"] == "passed"
        assert report["validated"] == len(report["cells"])
        for cell in report["cells"]:
            version, topology = cell["cell"].removeprefix("pg").split("-", 1)
            key = ("postgres", version, topology)
            assert key not in verified
            verified[key] = (cell, name, report["checked_at"])
    family_cells = {}
    mysql_family = LIVE / "mysql-family-check.json"
    if mysql_family.exists():
        report = read(mysql_family.name)
        family_cells = {(c["database"], c["version"], c["topology"]): c for c in report["catalog_cells"]}
        for cell in report["catalog_cells"]:
            if cell["status"] != "passed":
                continue
            key = (cell["database"], cell["version"], cell["topology"])
            assert key not in verified
            verified[key] = (cell, mysql_family.name, cell["checked_at"])
    rows = []
    clusters = read("mysql-clusters-check.json") if (LIVE / "mysql-clusters-check.json").exists() else {}
    cluster_cells = {(c["database"], c["version"], c["topology"]): c for c in clusters.get("catalog_cells", [])}
    for key, cell in cluster_cells.items():
        if cell.get("status") == "passed":
            assert key not in verified
            verified[key] = (cell, "mysql-clusters-check.json", cell["checked_at"])
    functional_name = "catalog-functional-check.json"
    functional = read(functional_name) if (LIVE / functional_name).exists() else {}
    functional_cells = {(c["database"], c["version"], c["topology"]): c for c in functional.get("catalog_cells", [])}
    for key, cell in functional_cells.items():
        if cell.get("status") == "passed":
            assert key not in verified
            verified[key] = (cell, functional_name, cell["checked_at"])
    for item in read("matrix/inventory.json"):
        key = (item["database"], item["version"], item["topology"])
        proof = verified.get(key)
        cell, evidence, checked = proof or ({}, "", "")
        row = dict(database=key[0], version=key[1], topology=key[2],
                   provider="yandex", zone=",".join(cell.get("zones", ["ru-central1-a"])), workload=item["script"],
                   database_actual_version=cell.get("database_actual_version", ""),
                   started_at=cell.get("started_at", ""), finished_at=cell.get("finished_at", ""),
                   preset="smoke-2m-2vus", compile="blocked" if "error" in item else "passed",
                   smoke="passed" if proof else "pending", full_workloads="pending",
                   baseline_matrix="pending", segment_matrix="pending",
                   component_metrics="passed" if proof else "pending",
                   managed_database_metrics="pending" if key[0] == "ydb_managed" else "not_applicable",
                   database_metric_distributions="not_applicable" if key[0] in ("noop", "pg_noop") else "pending",
                   component_logs=cell.get("component_logs", "pending"),
                   artifacts=cell.get("artifacts", "pending"),
                   replication=cell.get("replication", "pending"),
                   pipeline_traces="passed" if proof else "pending",
                   native_tps="not_applicable", native_otlp_metrics=cell.get("native_otlp_metrics", "pending"),
                   fault_campaign="pending", cleanup="passed" if proof else "pending",
                   run_id=cell.get("run_id", ""), iterations=cell.get("iterations", ""),
                   input=cell.get("input", "matrix/" + item["input"] if "input" in item else ""),
                   evidence=evidence, checked_at=checked, note=item.get("error", ""))
        if key[0] == "noop":
            row["note"] = "simple requires result rows; use execute_sql; earlier noop lifecycle smoke is documented in README"
        if key[0] == "external":
            row["note"] = "DSN placeholder; requires a real external database"
        family = family_cells.get(key)
        if family and not proof:
            for field in ("component_metrics", "component_logs", "artifacts", "replication", "pipeline_traces", "native_otlp_metrics", "cleanup", "run_id", "input", "checked_at", "note"):
                row[field] = family.get(field, row[field])
            row["smoke"] = family["status"]
            row["evidence"] = "mysql-family-check.json"
        cluster = cluster_cells.get(key)
        row["component_log_observations"] = json.dumps(cell.get("log_observations")) if "log_observations" in cell else "not_checked"
        row["retry_attempts"] = cell.get("retry_attempts", "")
        row["local_cluster"] = cluster.get("local_cluster", "pending") if cluster else "not_applicable"
        if cluster:
            row["compile"] = cluster["compile"]
            row["note"] = cluster.get("note", "")
            row["input"] = cluster.get("input", row["input"])
            if not proof:
                row["evidence"] = "mysql-clusters-check.json"
                row["run_id"] = cluster.get("run_id", "")
        functional_cell = functional_cells.get(key)
        if functional_cell:
            for field in ("compile", "component_metrics", "component_logs", "artifacts", "replication", "pipeline_traces", "native_otlp_metrics", "cleanup", "run_id", "input", "checked_at", "note", "local_cluster"):
                row[field] = functional_cell.get(field, row[field])
            row["smoke"] = functional_cell["status"]
            row["zone"] = ",".join(functional_cell.get("zones", [row["zone"]]))
            row["evidence"] = functional_name
            if key[0] in ("noop", "pg_noop", "external"):
                row["workload"] = "execute_sql"
        rows.append(row)
    assert len(verified) == sum(r["smoke"] == "passed" for r in rows)
    for native_name, cells_key in (("native-export-check.json", "cells"), ("mysql-family-check.json", "native_cells"), ("mysql-clusters-check.json", "native_cells"), (functional_name, "native_cells")):
        if not (LIVE / native_name).exists():
            continue
        native = read(native_name)
        for cell in native.get(cells_key, []):
            row = {key: "" for key in rows[0]}
            row.update(database=cell["database"], version=cell["version"], topology=cell["topology"],
                       provider="yandex", zone=",".join(cell.get("zones", ["ru-central1-a"])), workload=cell["workload"],
                       database_actual_version=cell.get("database_actual_version", ""),
                       started_at=cell.get("started_at", ""), finished_at=cell.get("finished_at", ""),
                       preset=cell.get("preset", "native-otlp-20s-2vus"), compile="passed", smoke=cell["status"],
                       full_workloads="pending", baseline_matrix="pending", segment_matrix="pending",
                       native_tps=cell["native_tps"], native_otlp_metrics=cell["native_otlp_metrics"],
                       component_metrics=cell.get("component_metrics", "pending"),
                       managed_database_metrics="pending" if cell["database"] == "ydb_managed" else "not_applicable",
                       database_metric_distributions="not_applicable" if cell["database"] in ("noop", "pg_noop") else "pending",
                       component_logs=cell.get("component_logs", "pending"),
                       artifacts=cell.get("artifacts", "pending"),
                       replication=cell.get("replication", "pending"),
                       pipeline_traces=cell.get("pipeline_traces", "pending"), fault_campaign="pending",
                       cleanup=cell.get("cleanup", "pending"), run_id=cell["run_id"],
                       iterations=cell.get("iterations", ""), input=cell.get("input", "suite-native-exports.json"),
                       evidence=native_name, checked_at=cell.get("checked_at", native.get("checked_at", "")),
                       component_log_observations=json.dumps(cell.get("log_observations")) if "log_observations" in cell else "not_checked", retry_attempts=cell.get("retry_attempts", ""),
                       note="segment="+cell["segment"]+("; "+cell["note"] if cell.get("note") else ""))
            rows.append(row)
    if (LIVE / "current-campaigns.json").exists():
        apply_current_campaigns(rows, read("current-campaigns.json"))
    recovery_file = LIVE / "yc-vm-recovery-check.json"
    recovered = {}
    if recovery_file.exists():
        proof = read(recovery_file.name)
        for item in proof.get("workflow_runs", []):
            recovered[item["run_id"]] = item["recovered_vm_count"]
    automatic_file = LIVE / "yc-autonomous-provisioning-check.json"
    automatic = read(automatic_file.name) if automatic_file.exists() else {}
    automatic_runs = {r["run_id"] for r in automatic.get("workflow_runs", [])} if automatic.get("status") == "passed" else set()
    for row in rows:
        count = recovered.get(row.get("run_id"))
        row["infrastructure_provisioning"] = "automatic_verified" if row.get("run_id") in automatic_runs else "manual_recovery" if count else "not_recorded"
        row["infrastructure_recovery"] = f"manual_vm_links:{count}" if count else ""
        row["recovery_evidence"] = recovery_file.name if count else ""
    output = LIVE / "progress.csv"
    with tempfile.NamedTemporaryFile(mode="w", newline="", encoding="utf-8",
                                     dir=LIVE, prefix=".progress-", delete=False) as handle:
        writer = csv.DictWriter(handle, fieldnames=list(rows[0]), lineterminator="\n")
        writer.writeheader()
        writer.writerows(rows)
        temporary = Path(handle.name)
    os.chmod(temporary, 0o644)
    temporary.replace(output)
    print(f"{output}: {len(rows)} rows, {sum(r['smoke'] == 'passed' for r in rows)} verified checks")

if __name__ == "__main__":
    raise SystemExit("Historical generator. Use tools/live.py progress for the current ledger.")
