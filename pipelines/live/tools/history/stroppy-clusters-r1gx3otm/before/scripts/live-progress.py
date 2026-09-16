#!/usr/bin/env python3
"""Rebuild the reviewable YC smoke matrix from saved verification evidence."""
import csv
import json
from pathlib import Path

LIVE = Path(__file__).resolve().parents[1] / "pipelines/live"

def read(name):
    return json.loads((LIVE / name).read_text())

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
    mysql_family = LIVE / "mysql-family-check.json"
    if mysql_family.exists():
        report = read(mysql_family.name)
        for cell in report["catalog_cells"]:
            if cell["status"] != "passed":
                continue
            key = (cell["database"], cell["version"], cell["topology"])
            assert key not in verified
            verified[key] = (cell, mysql_family.name, cell["checked_at"])
    rows = []
    for item in read("matrix/inventory.json"):
        key = (item["database"], item["version"], item["topology"])
        proof = verified.get(key)
        cell, evidence, checked = proof or ({}, "", "")
        row = dict(database=key[0], version=key[1], topology=key[2],
                   provider="yandex", zone="ru-central1-a", workload=item["script"],
                   database_actual_version=cell.get("database_actual_version", ""),
                   started_at=cell.get("started_at", ""), finished_at=cell.get("finished_at", ""),
                   preset="smoke-2m-2vus", compile="blocked" if "error" in item else "passed",
                   smoke="passed" if proof else "pending", full_workloads="pending",
                   baseline_matrix="pending", segment_matrix="pending",
                   component_metrics="passed" if proof else "pending",
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
        rows.append(row)
    assert len(verified) == sum(r["smoke"] == "passed" for r in rows)
    for native_name, cells_key in (("native-export-check.json", "cells"), ("mysql-family-check.json", "native_cells")):
        if not (LIVE / native_name).exists():
            continue
        native = read(native_name)
        for cell in native[cells_key]:
            row = {key: "" for key in rows[0]}
            row.update(database=cell["database"], version=cell["version"], topology=cell["topology"],
                       provider="yandex", zone="ru-central1-a", workload=cell["workload"],
                       database_actual_version=cell.get("database_actual_version", ""),
                       started_at=cell.get("started_at", ""), finished_at=cell.get("finished_at", ""),
                       preset=cell.get("preset", "native-otlp-20s-2vus"), compile="passed", smoke=cell["status"],
                       full_workloads="pending", baseline_matrix="pending", segment_matrix="pending",
                       native_tps=cell["native_tps"], native_otlp_metrics=cell["native_otlp_metrics"],
                       component_metrics=cell.get("component_metrics", "pending"),
                       component_logs=cell.get("component_logs", "pending"),
                       artifacts=cell.get("artifacts", "pending"),
                       replication=cell.get("replication", "pending"),
                       pipeline_traces=cell.get("pipeline_traces", "pending"), fault_campaign="pending",
                       cleanup=cell.get("cleanup", "pending"), run_id=cell["run_id"],
                       iterations=cell.get("iterations", ""), input=cell.get("input", "suite-native-exports.json"),
                       evidence=native_name, checked_at=cell.get("checked_at", native.get("checked_at", "")),
                       note="segment="+cell["segment"])
            rows.append(row)
    output = LIVE / "progress.csv"
    with output.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)
    print(f"{output}: {len(rows)} rows, {sum(r['smoke'] == 'passed' for r in rows)} verified checks")

if __name__ == "__main__":
    main()
