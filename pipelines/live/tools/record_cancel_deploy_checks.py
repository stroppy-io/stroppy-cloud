#!/usr/bin/env python3
"""Record cancellation acceptance from saved server evidence, without launching work."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load(path):
    return json.loads(path.read_text())


def walk(node):
    yield node
    for child in node.get("children", []):
        yield from walk(child)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--case", required=True)
    parser.add_argument("--campaign", required=True)
    args = parser.parse_args()
    case = ROOT / "tests" / args.case
    campaign = ROOT / "tests/platform/server/resilience" / args.campaign
    checks = load(case / "checks.json")
    run_id = checks["run_id"]
    run_dir = case / "runs" / run_id
    before = load(campaign / "before-cancel.json")["body"]
    final = load(run_dir / "run.json")["body"]
    tree = load(campaign / "tree-before-cancel.json")["body"]
    cleanup = load(campaign / "cleanup.json")
    events = load(run_dir / "events.json")["body"]
    assert before["id"] == final["id"] == cleanup["run_id"] == run_id
    assert tree["ref"] == "run/" + run_id
    assert before["status"] == "running" and before["phase"] == "deploying"
    assert final["status"] == "cancelled" and final["phase"] == "done"
    assert not final.get("result", {}).get("segments", [])
    assert cleanup["verified"] and not cleanup["remaining_infrastructure"]
    assert not events["meta"]["has_more"]
    assert not any(e["kind"] == "segment.started" for e in events["data"])
    expected = {m["name"] for m in before["snapshot"]["machines"]}
    ready = {e["subject"] for e in events["data"] if e["kind"] == "machine.ready"}
    instances = [n for n in walk(tree) if n["kind"].endswith(".Instance")]
    assert len(expected) > 1 and ready == expected
    assert len(instances) == len(expected) and all(n["phase"] == "ready" for n in instances)
    proof = {
        "run_id": run_id, "verified": True, "ready_machines": sorted(ready),
        "cancelled_phase": before["phase"], "terminal_status": final["status"],
        "cleanup_verified": True,
        "scope": "Multi-machine deployment cancellation only. SQL, replication and workload telemetry were not exercised.",
    }
    (campaign / "verification.json").write_text(json.dumps(proof, indent=2) + "\n")
    updates = {
        "infrastructure_provisioning": "All expected machines were ready before cancellation. Exact hardware conformance is not asserted.",
        "smoke": "Lifecycle regression: cancelled during deployment, reached cancelled/done without workload segments. Not a SQL smoke test.",
        "cleanup": "Server resource tree contains no remaining infrastructure after cancellation.",
    }
    for check in checks["checks"]:
        if check["id"] in updates:
            check.update(status="passed", reason=updates[check["id"]], evidence={
                "path": str((campaign / "verification.json").relative_to(ROOT)), "pointer": "/verified",
            })
    checks["checked_at"] = datetime.now(timezone.utc).isoformat()
    (case / "checks.json").write_text(json.dumps(checks, indent=2) + "\n")
    print(json.dumps(proof))


if __name__ == "__main__":
    main()
