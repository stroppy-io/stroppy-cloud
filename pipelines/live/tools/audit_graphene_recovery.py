#!/usr/bin/env python3
"""Read-only source evidence for the Graphene recovery contract, not a live test."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import subprocess


def capture(root, path, signature):
    raw = (root / path).read_bytes()
    source = raw.decode()
    start = source.index(signature)
    end = source.find("\nfunc ", start + len(signature))
    fragment = source[start:end if end != -1 else len(source)]
    return {
        "file": path,
        "line": source[:start].count("\n") + 1,
        "file_sha256": hashlib.sha256(raw).hexdigest(),
        "source": fragment.rstrip(),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--graphene", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    root = args.graphene.resolve()
    fragments = {
        "management_get_run": capture(root, "internal/services/management.go", "func (m *Management) GetRun("),
        "worker_get_run": capture(root, "internal/services/workerplane.go", "func (w *WorkerPlane) GetRun("),
        "start_core": capture(root, "internal/services/management.go", "func startRunCore("),
        "fire_run": capture(root, "internal/services/management.go", "func fireRun("),
        "arbiter": capture(root, "internal/pipelineflow/pipelineflow.go", "func New("),
    }
    checks = {
        "management_maps_all_describe_errors_to_not_found": 'if err != nil {\n\t\treturn nil, status.Error(codes.NotFound, err.Error())' in fragments["management_get_run"]["source"],
        "worker_maps_all_describe_errors_to_not_found": 'if err != nil {\n\t\treturn nil, status.Error(codes.NotFound, err.Error())' in fragments["worker_get_run"]["source"],
        "start_permits_reuse_after_failure": "WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY" in fragments["start_core"]["source"],
        "queued_firing_returns_failed_precondition": '"run %s: the pipeline\'s concurrency policy queued this firing behind the live run"' in fragments["fire_run"]["source"],
        "arbiter_has_single_pending_slot": "st.Pending = &fire" in fragments["arbiter"]["source"],
    }
    git = lambda *a: subprocess.check_output(["git", "-C", str(root), *a], text=True).strip()
    report = {
        "observed_at": datetime.now(timezone.utc).isoformat(),
        "evidence_kind": "source_characterization",
        "live_fault_injection": False,
        "head": git("rev-parse", "HEAD"),
        "v0.2.23": git("rev-parse", "v0.2.23"),
        "working_tree_clean": not bool(git("status", "--porcelain")),
        "checks": checks,
        "fragments": fragments,
        "note": "True means the problematic implementation pattern is present, not that recovery passed. This tool reads source only and sends no Graphene or cloud requests.",
    }
    args.evidence.parent.mkdir(parents=True, exist_ok=True)
    args.evidence.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"head": report["head"], "checks": checks, "evidence": str(args.evidence)}))


if __name__ == "__main__":
    main()
