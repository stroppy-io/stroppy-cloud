#!/usr/bin/env python3
"""A resumable server-only suite campaign; local Compose/proxy inject outages.

--start creates two tests and one suite. --watch observes the saved launch;
it never starts another suite. Proxy faults expire automatically.
"""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import subprocess
import time
import urllib.request
import uuid

from server_api import Client
from server_live_acceptance import ROOT, BASE, TERMINAL

DEST = ROOT / "pipelines/live/tests/platform/server/resilience/live-suite-20260927"
CONTROL = "http://127.0.0.1:18349"


def save(name, value):
    DEST.mkdir(parents=True, exist_ok=True)
    (DEST / name).write_text(json.dumps(value, indent=2) + "\n")


def request(path, method="GET", data=None, name=None, key=None):
    r = Client().request(BASE + path, method, data,
                         evidence=DEST/name if name else None, idempotency_key=key, timeout=40)
    if r["http_status"] not in {200, 201, 202, 204}:
        raise RuntimeError(f"{method} {path}: HTTP {r['http_status']}, {r['body'].get('code')}")
    return r["body"]


def fault(seconds):
    data = json.dumps({"method": "*", "seconds": seconds}).encode()
    req = urllib.request.Request(CONTROL + "/fault", data=data,
                                 headers={"Content-Type": "application/json"}, method="POST")
    with urllib.request.urlopen(req, timeout=5) as response:
        assert response.status == 204


def stats(name):
    with urllib.request.urlopen(CONTROL + "/stats", timeout=5) as response:
        save(name, json.load(response))


def start():
    if (DEST / "launch-intent.json").exists():
        raise RuntimeError("existing campaign: inspect saved intent; do not start another suite")
    tests = []
    for kind, source in [("noop", "0bae3342-5c1c-4c4e-9105-decf257caa68"),
                         ("postgres", "72872cd7-c89e-448b-a77b-c206b1dc40c9")]:
        clone = request(f"/tests/{source}:clone", "POST", {"name": "resilience-20260927-"+kind}, kind+"-created.json")
        workload = clone["workload"]
        segment = workload["inline"]["segments"][0]
        segment["run"]["duration"] = "2m"
        segment["log_level"] = "warn"
        request("/tests/"+clone["id"], "PATCH", {"workload": workload, "keep": "1m"}, kind+"-test.json")
        tests.append({"ref": {"id": clone["id"]}})
    suite = request("/suites", "POST", {"name": "resilience outage restart keep 20260927", "tests": tests, "concurrency": 2}, "suite.json")
    key = str(uuid.uuid4())
    path = "/suites/"+suite["id"]+":launch"
    save("launch-intent.json", {"path": path, "idempotency_key": key})
    fault(45)
    try:
        first = request(path, "POST", {}, "launch.json", key)
        stats("submission-outage.json")
    finally:
        fault(0)
    again = request(path, "POST", {}, "launch-repeated.json", key)
    assert first["id"] == again["id"], "duplicate suite row"
    save("submission-verification.json", {"suite_id": first["id"], "same_server_identity": first["id"] == again["id"],
                                           "first_status": first["status"], "observed_at": datetime.now(timezone.utc).isoformat()})
    print("suite launched", first["id"], "status", first["status"], flush=True)


def watch():
    sid = json.loads((DEST/"launch.json").read_text())["body"]["id"]
    restarted = (DEST/"restart-intent.json").exists()
    last = None
    history = json.loads((DEST/"observations.json").read_text()) if (DEST/"observations.json").exists() else []
    deadline = time.monotonic() + 2400
    while time.monotonic() < deadline:
        try:
            suite = request("/suite-runs/"+sid)
            runs = [request("/runs/"+cell["run"]["id"]) for cell in suite["cells"]]
        except (OSError, RuntimeError):
            time.sleep(5)
            continue
        state = (suite["status"], [(r["id"], r["status"], r.get("phase"), r.get("stand_kept"), r.get("keep_until")) for r in runs])
        if state != last:
            print(state, flush=True)
            history.append({"at": datetime.now(timezone.utc).isoformat(), "state": state})
            save("observations.json", history)
            last = state
        if not restarted and any(r["status"] == "running" and r.get("phase") == "workload" for r in runs):
            save("restart-intent.json", {"at": datetime.now(timezone.utc).isoformat(), "suite_id": sid,
                                         "runs": [{"id": r["id"], "graphene": r["graphene"]} for r in runs]})
            fault(30)
            subprocess.run(["docker", "compose", "--env-file", ".env.compose", "restart", "server"], cwd=ROOT, check=True)
            restarted = True
            stats("restart-outage.json")
        for r in runs:
            if r.get("stand_kept"):
                save("kept-"+r["id"]+".json", r)
        if suite["status"] in TERMINAL and all(r["status"] in TERMINAL and not r.get("stand_kept") for r in runs):
            save("suite-final.json", suite)
            cleaned = True
            for r in runs:
                save("run-"+r["id"]+".json", r)
                result = subprocess.run(["python3", str(ROOT/"pipelines/live/tools/verify_server_cleanup.py"), "--tenant", "server-live-20260924",
                                         "--run-id", r["id"], "--evidence", str(DEST/("cleanup-"+r["id"]+".json"))], capture_output=True)
                cleaned = cleaned and result.returncode == 0
            if not cleaned:
                time.sleep(5)
                continue
            before = json.loads((DEST/"restart-intent.json").read_text()) if restarted else {"runs": []}
            same = before["runs"] == [{"id": r["id"], "graphene": r["graphene"]} for r in runs]
            cancellation = json.loads((DEST/"cancel-intent.json").read_text()) if (DEST/"cancel-intent.json").exists() else {}
            ok = restarted and same and suite["status"] == "completed" and all(
                r["status"] == "cancelled" if r["id"] == cancellation.get("run_id") else
                r["status"] == "completed" and r.get("result", {}).get("segments") and (DEST/("kept-"+r["id"]+".json")).exists() for r in runs)
            save("verification.json", {"verified": ok, "same_server_and_graphene_references": same, "server_restarted": restarted,
                                       "suite_id": sid, "intentional_cancellation": cancellation,
                                       "note": "PostgreSQL success/keep expiry and optional noop deployment cancellation; resource cleanup through server tree. Temporal execution UUID is not exposed by this API."})
            if not ok:
                raise RuntimeError("resilience acceptance incomplete; inspect evidence")
            return
        time.sleep(5)
    raise TimeoutError("campaign still active: continue observing the saved suite")


def retry_launch():
    intent = json.loads((DEST/"launch-intent.json").read_text())
    old = json.loads((DEST/"launch.json").read_text())
    if old["http_status"] != 503:
        raise RuntimeError("retry-launch expects the recorded pre-admission outage")
    save("launch-unavailable.json", old)
    first = request(intent["path"], "POST", {}, "launch.json", intent["idempotency_key"])
    again = request(intent["path"], "POST", {}, "launch-repeated.json", intent["idempotency_key"])
    assert first["id"] == again["id"], "duplicate suite row"
    save("submission-verification.json", {"suite_id": first["id"], "same_server_identity": True,
                                           "outage_boundary": "provider quota check before admission; HTTP 503, no suite created",
                                           "first_status": first["status"]})
    print("suite launched after recovery", first["id"], flush=True)


def cancel_noop():
    sid = json.loads((DEST/"launch.json").read_text())["body"]["id"]
    suite = request("/suite-runs/"+sid)
    cell = next(c for c in suite["cells"] if c["name"].endswith("noop"))
    rid = cell["run"]["id"]
    before = request("/runs/"+rid, name="noop-before-cancel.json")
    if before["status"] != "running" or before["phase"] != "deploying":
        raise RuntimeError("cancel scenario expects an active deployment")
    request("/runs/"+rid+"/overview", name="noop-deployment-before-cancel.json")
    save("cancel-intent.json", {"run_id": rid, "at": datetime.now(timezone.utc).isoformat(),
                               "reason": "Exercise durable cancellation during deployment. Slow image pull remains unqualified."})
    fault(30)
    try:
        r = Client().request(BASE+"/runs/"+rid+":cancel", "POST", evidence=DEST/"cancel-unavailable.json")
        assert r["http_status"] == 503, "expected cancel delivery outage"
        local = request("/runs/"+rid, name="cancel-persisted.json")
        assert local["status"] == "cancelling", "cancel command was not persisted"
        stats("cancel-outage.json")
    finally:
        fault(0)
    print("cancellation persisted; projector must deliver it after recovery", rid, flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--start", action="store_true")
    parser.add_argument("--watch", action="store_true")
    parser.add_argument("--retry-launch", action="store_true")
    parser.add_argument("--cancel-noop", action="store_true")
    args = parser.parse_args()
    if not (args.start or args.watch or args.retry_launch or args.cancel_noop): parser.error("choose an action")
    if args.start: start()
    if args.retry_launch: retry_launch()
    if args.cancel_noop: cancel_noop()
    if args.watch: watch()
