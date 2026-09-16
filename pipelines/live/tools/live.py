#!/usr/bin/env python3
"""Inspect and rebuild the offline live-test ledger. Never launches resources."""

import argparse
import csv
import hashlib
import json
import os
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
GROUPS = ("deployment", "workload_check", "native_metrics", "telemetry", "artifacts", "replication", "cleanup")
FIELDS = ("database", "version", "topology", "workload", "preset", "execution", *GROUPS, "readiness", "next_action", "details")
STATUSES = {"passed", "failed", "not_run", "unknown", "blocked", "not_applicable"}
REQUIRED_CHECKS = {
    "compile": "deployment", "infrastructure_provisioning": "deployment",
    "smoke": "workload_check", "native_tps": "native_metrics", "native_otlp_metrics": "native_metrics",
    "component_metrics": "telemetry", "component_logs": "telemetry", "pipeline_traces": "telemetry",
    "database_metric_distributions": "telemetry", "managed_database_metrics": "telemetry",
    "artifacts": "artifacts", "replication": "replication", "cleanup": "cleanup",
}


def read(path):
    return json.loads(path.read_text())


def inside(root, path):
    resolved = (root / path).resolve()
    if not resolved.is_relative_to(root.resolve()):
        raise ValueError(f"Path escapes live directory: {path}")
    return resolved


def resolve(root, name):
    """Resolve current paths and retained historical filenames without guessing."""
    direct = inside(root, name)
    if direct.exists():
        return direct
    paths = read(root / "tests/platform/migration/paths.json")
    if name not in paths:
        raise ValueError(f"Unknown evidence path: {name}")
    return inside(root, paths[name])


def reference(root, ref):
    path = resolve(root, ref["path"])
    if not path.is_file():
        raise ValueError(f"Missing evidence: {ref['path']}")
    pointer = ref.get("pointer", "")
    if pointer:
        if not pointer.startswith("/"):
            raise ValueError(f"Invalid JSON pointer: {pointer}")
        value = read(path)
        for part in pointer[1:].split("/"):
            key = part.replace("~1", "/").replace("~0", "~")
            value = value[int(key)] if isinstance(value, list) else value[key]
    return path


def cases(root):
    for path in sorted(root.glob("tests/*/*/*/*/*/case.json")):
        if path.parts[-6] in {"tools", "platform", "bootstrap"}:
            continue
        case = read(path)
        checks = read(inside(root, str(path.parent.relative_to(root) / case["checks"])))
        yield path, case, checks


def aggregate(checks):
    """Unknown or missing checks never count as successes; N/A is excluded."""
    applicable = [c for c in checks if c["status"] != "not_applicable"]
    if not checks:
        return "unknown 0/0"
    if not applicable:
        return "not_applicable"
    states = {c["status"] for c in applicable}
    passed = sum(c["status"] == "passed" for c in applicable)
    status = next((s for s in ("failed", "blocked", "unknown", "not_run") if s in states), "passed")
    return f"{status} {passed}/{len(applicable)}"


def table_rows(root):
    for path, case, record in cases(root):
        checks = record["checks"]
        row = {k: case[k] for k in FIELDS[:5]}
        row["execution"] = case["execution"]["status"]
        row.update({g: aggregate([c for c in checks if c["group"] == g]) for g in GROUPS})
        row["readiness"] = aggregate(checks)
        outstanding = [c for c in checks if c["status"] not in {"passed", "not_applicable"}]
        outstanding.sort(key=lambda c: ({"failed": 0, "blocked": 1, "unknown": 2, "not_run": 3}[c["status"]], c["id"]))
        row["next_action"] = "; ".join(f"{c['id']}: {c['status']}" for c in outstanding[:2])
        if len(outstanding) > 2:
            row["next_action"] += f"; +{len(outstanding)-2} (see details)"
        if case["execution"]["status"] == "not_started":
            row["next_action"] = "Draft: replan before launching"
        if not outstanding:
            row["next_action"] = "Case checks passed; full matrix scope remains separate"
        row["details"] = str(path.parent.relative_to(root / "tests") / "README.md")
        yield row


def validate(root):
    """Validate identity, evidence pointers, shared runs and migration integrity."""
    ids = set()
    for path, case, record in cases(root):
        rel = str(path.parent.relative_to(root))
        if case["id"] != rel or rel in ids:
            raise ValueError(f"Invalid or duplicate case identity: {rel}")
        ids.add(rel)
        if record["run_id"] != case["selected_run"]:
            raise ValueError(f"Checks belong to a different attempt: {rel}")
        seen = set()
        for check in record["checks"]:
            if check["id"] in seen or check["status"] not in STATUSES or check["group"] not in GROUPS:
                raise ValueError(f"Invalid check: {rel}/{check['id']}")
            seen.add(check["id"])
            if not check.get("reason"):
                raise ValueError(f"Missing check explanation: {rel}/{check['id']}")
            if check["status"] == "passed" and not check.get("evidence"):
                raise ValueError(f"Passed without evidence: {rel}/{check['id']}")
            if check.get("evidence"):
                reference(root, check["evidence"])
        if set(GROUPS) - {c["group"] for c in record["checks"]}:
            raise ValueError(f"Missing check groups: {rel}")
        if set(REQUIRED_CHECKS) - seen:
            raise ValueError(f"Missing required checks: {rel}")
        for check in record["checks"]:
            if check["id"] in REQUIRED_CHECKS and check["group"] != REQUIRED_CHECKS[check["id"]]:
                raise ValueError(f"Incorrect check group: {rel}/{check['id']}")
        reference(root, case["source"])
        for run in case["runs"]:
            run_dir = inside(root, str(path.parent.relative_to(root) / run))
            manifest = read(run_dir / "manifest.json")
            if rel not in manifest["case_paths"] or manifest["run_id"] != Path(run).name:
                raise ValueError(f"Invalid run association: {rel}/{run}")
            if manifest.get("input_source"):
                reference(root, manifest["input_source"])
        if case["selected_run"] and "runs/" + case["selected_run"] not in case["runs"]:
            raise ValueError(f"Selected run missing: {rel}")
        if case.get("input") and not inside(root, str(path.parent.relative_to(root) / case["input"])).is_file():
            raise ValueError(f"Input missing: {rel}")
    for entry in read(root / "tests/platform/migration/files.json"):
        path = inside(root, entry["path"])
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        if digest != entry.get("current_sha256", entry["sha256"]):
            raise ValueError(f"Migrated file changed without provenance update: {entry['path']}")
    for old, new in read(root / "tests/platform/migration/paths.json").items():
        if not inside(root, new).is_file():
            raise ValueError(f"Broken migration reference: {old} -> {new}")
    for path in root.rglob("*"):
        if path.is_symlink():
            if not path.exists() or not path.resolve().is_relative_to(root.resolve()):
                raise ValueError(f"Invalid shared run link: {path}")
    if not ids:
        raise ValueError("No test cases found")
    return len(ids)


def relative_link(path, target, label):
    return f"[{label}]({os.path.relpath(target, path.parent)})"


def render_details(root, path, case, record):
    lines = [f"# {case['database']} {case['version']} / {case['topology']} / {case['workload']} / {case['preset']}", "",
             f"Область проверки: {case['acceptance_scope']}", "",
             f"Состояние исполнения: `{case['execution']['status']}`. Наблюдалось: `{case['execution'].get('observed_at') or 'не запускался'}`.", "",
             "[Паспорт случая](case.json) · [Проверки](checks.json)", ""]
    if case.get("input"):
        lines += [f"[Вход]({case['input']}) — {case.get('input_note', case['input_status'])}", ""]
    lines += ["| Проверка | Статус | Основание |", "|---|---|---|"]
    for check in record["checks"]:
        evidence = check.get("evidence")
        proof = relative_link(path, reference(root, evidence), "доказательство") if evidence else check["reason"]
        if evidence and evidence.get("pointer"):
            proof += f" (`{evidence['pointer']}`)"
        lines.append(f"| {check['id']} | {check['status']} | {proof.replace('|', '/') } |")
    lines += ["", "Прогоны:", ""]
    lines += [f"- [{run}]({run}/manifest.json)" for run in case["runs"]] or ["Прогонов пока нет."]
    historical = path.parent / "history"
    if historical.is_dir():
        lines += ["", "Предыдущие материалы (не текущее подтверждение):", ""]
        lines += [f"- [{item.name}](history/{item.name})" for item in sorted(historical.iterdir()) if item.is_file()]
    lines += ["", relative_link(path, root / "tests/progress.csv", "Единая таблица"), ""]
    path.write_text("\n".join(lines))


def progress(root):
    count = validate(root)
    for path, case, record in cases(root):
        render_details(root, path.parent / "README.md", case, record)
    output = root / "tests/progress.csv"
    with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", newline="", dir=root, prefix=".progress-", delete=False) as handle:
        writer = csv.DictWriter(handle, fieldnames=FIELDS, lineterminator="\n")
        writer.writeheader()
        writer.writerows(table_rows(root))
        temporary = Path(handle.name)
    os.chmod(temporary, 0o644)
    temporary.replace(output)
    return count


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT)
    subs = parser.add_subparsers(dest="command", required=True)
    subs.add_parser("validate")
    subs.add_parser("progress")
    lookup = subs.add_parser("resolve")
    lookup.add_argument("name")
    args = parser.parse_args()
    if args.command == "resolve":
        print(resolve(args.root, args.name))
    elif args.command == "validate":
        print(f"Validated {validate(args.root)} cases")
    else:
        print(f"Updated progress.csv: {progress(args.root)} cases")


if __name__ == "__main__":
    main()
