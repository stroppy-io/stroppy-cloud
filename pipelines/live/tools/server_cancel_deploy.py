#!/usr/bin/env python3
"""Launch a known test and cancel during deploy, retaining a durable launch identity."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import time
import uuid

from server_api import Client


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--tenant", required=True)
    p.add_argument("--test-id", required=True)
    p.add_argument("--evidence-dir", required=True)
    a = p.parse_args()
    dest = Path(a.evidence_dir)
    dest.mkdir(parents=True, exist_ok=True)
    c, base = Client(), f"/api/v1/t/{a.tenant}"
    intent = dest/"launch-intent.json"
    if not intent.exists():
        intent.write_text(json.dumps({"test_id": a.test_id, "tenant": a.tenant, "key": str(uuid.uuid4())}, indent=2)+"\n")
    plan = json.loads(intent.read_text())
    assert plan["test_id"] == a.test_id and plan["tenant"] == a.tenant
    launched = dest/"launch.json"
    if not launched.exists():
        r = c.request(base+"/tests/"+a.test_id+":launch", "POST", {}, idempotency_key=plan["key"], evidence=dest/"launch-response.json")
        assert r["http_status"] == 201, r["http_status"]
        launched.write_text(json.dumps(r, indent=2)+"\n")
    rid = json.loads(launched.read_text())["body"]["id"]
    print("run", rid, flush=True)
    observations = dest/"observations.json"
    history = json.loads(observations.read_text()) if observations.exists() else []
    last = None
    deadline = time.monotonic()+1800
    while time.monotonic() < deadline:
        r = c.request(base+"/runs/"+rid, timeout=30)
        assert r["http_status"] == 200, r["http_status"]
        run = r["body"]
        state = [run["status"], run["phase"]]
        if state != last:
            history.append({"observed_at": datetime.now(timezone.utc).isoformat(), "state": state})
            observations.write_text(json.dumps(history, indent=2)+"\n")
            print(state, flush=True)
            last = state
        if state == ["running", "deploying"] and not (dest/"cancel.json").exists():
            (dest/"before-cancel.json").write_text(json.dumps(r, indent=2)+"\n")
            c.request(base+"/runs/"+rid+"/tree", evidence=dest/"tree-before-cancel.json")
            response = c.request(base+"/runs/"+rid+":cancel", "POST", {}, evidence=dest/"cancel-response.json")
            if response["http_status"] == 200:
                (dest/"cancel.json").write_text(json.dumps(response, indent=2)+"\n")
                print("cancel accepted during deploy", flush=True)
            elif response["http_status"] < 500:
                raise RuntimeError(f"cancel HTTP {response['http_status']}")
        if run["status"] in {"completed", "failed", "cancelled"}:
            (dest/"final.json").write_text(json.dumps(r, indent=2)+"\n")
            segments = run.get("result", {}).get("segments", [])
            report = {"run_id": rid, "status": run["status"], "segment_count": len(segments),
                      "verified": run["status"] == "cancelled" and not segments and (dest/"cancel.json").exists(),
                      "scope": "Cancellation lifecycle only; infrastructure removal must be checked separately."}
            (dest/"lifecycle.json").write_text(json.dumps(report, indent=2)+"\n")
            print(json.dumps(report), flush=True)
            return 0 if report["verified"] else 1
        if run["phase"] == "workload" and not (dest/"cancel.json").exists():
            # Do not pretend this hit the required phase. Stop the owned test
            # to avoid unnecessary workload, then report failed coverage.
            c.request(base+"/runs/"+rid+":cancel", "POST", {}, evidence=dest/"late-cancel.json")
        time.sleep(5)
    raise TimeoutError("run still active; inspect saved identity, never relaunch blindly")


if __name__ == "__main__":
    raise SystemExit(main())
