#!/usr/bin/env python3
"""Record an image-pull blocker after cancellation and verified cleanup."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--case", required=True, help="case path relative to live/tests")
    p.add_argument("--evidence", required=True, help="campaign evidence relative to live/tests")
    a = p.parse_args()
    case, evidence = ROOT/"tests"/a.case, ROOT/"tests"/a.evidence
    report = json.loads((case/"checks.json").read_text())
    run_dir = case/"runs"/report["run_id"]
    run = json.loads((run_dir/"run.json").read_text())["body"]
    cleanup = json.loads((evidence/"cleanup.json").read_text())
    logs = json.loads((evidence/"registry-errors.json").read_text())["body"]["data"]
    assert run["status"] == "cancelled" and not run.get("result", {}).get("segments")
    assert cleanup["verified"] and cleanup["run_id"] == report["run_id"]
    assert any("pull " in l.get("fields", {}).get("error", "") and
               any(word in l["fields"]["error"] for word in ["timeout", "deadline exceeded"])
               for l in logs)
    events = json.loads((run_dir/"events.json").read_text())["body"]["data"]
    for check in report["checks"]:
        path, pointer = None, None
        if check["id"] == "cleanup":
            check.update(status="passed", reason="Cancellation finished; server resource tree has no remaining infrastructure.")
            path, pointer = evidence/"cleanup.json", "/verified"
        elif check["id"] == "infrastructure_provisioning" and any(
                e.get("kind") == "phase.finished" and e.get("subject") == "provisioning" for e in events):
            check.update(status="passed", reason="Resource and agent readiness completed before deployment. Exact hardware SKU is not asserted.")
            path, pointer = run_dir/"events.json", "/body/data"
        elif check["id"] in {"smoke", "native_otlp_metrics", "artifacts"}:
            check.update(status="blocked", reason="Image pull repeatedly timed out; cancelled before any workload segment executed.")
            path, pointer = evidence/"registry-errors.json", "/body/data"
        if path:
            check["evidence"] = {"path": str(path.relative_to(ROOT)), "pointer": pointer}
    report["checked_at"] = datetime.now(timezone.utc).isoformat()
    (case/"checks.json").write_text(json.dumps(report, indent=2)+"\n")


if __name__ == "__main__": main()
