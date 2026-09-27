#!/usr/bin/env python3
"""Launch/observe the prepared analytical test via server, preserving its identity."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import time
import uuid

from server_api import Client
from server_live_acceptance import ROOT, BASE, TERMINAL

DEST = ROOT / "pipelines/live/tests/platform/server/resilience/analytical-20260927"


def save(name, body):
    DEST.mkdir(parents=True, exist_ok=True)
    (DEST/name).write_text(json.dumps(body, indent=2)+"\n")


def prepare_hub(mirror=False, generated=False):
    if (DEST/"test.json").exists():
        raise RuntimeError("test already prepared")
    c = Client()
    r = c.request(BASE+"/tests/c15cc868-560b-48e9-b248-6be987b54817:clone", "POST",
                  {"name": "server analytical pg17 " + ("tpcds generated " if generated else "tpch tpcds ") + ("mirror" if mirror else "hub") + " sf001 " + DEST.name})
    save("test-created.json", r)
    if r["http_status"] != 201: raise RuntimeError("clone failed")
    test = r["body"]
    execution = test["execution"]
    image = "docker.io/prom/node-exporter@sha256:1b4e4438faca4dd7e001dd445d161a4a2091b0fededa84093b3a8dfeae1f1be0"
    execution["containers"] = [{"name": n, "set": {"image": image}} for n in ["db-1-node-exporter", "runner-1-node-exporter"]]
    if mirror:
        for patch in execution["containers"]:
            patch["set"]["image"] = image.replace("docker.io/", "mirror.gcr.io/")
        execution["containers"] += [
            {"name": "db-1-postgres", "set": {"image": "mirror.gcr.io/library/postgres@sha256:d74eeac9a635390a49bc21bd49fccd973de707e2a53a76ac49b552b8712ec46f"}},
            {"name": "db-1-postgres-exporter", "set": {"image": "mirror.gcr.io/prometheuscommunity/postgres-exporter@sha256:fb96c4413985d4b23ab02b19022b3d70a86c8e0a62f41ab15ebb6f4673781a5d"}},
        ]
    patch = {"execution": execution}
    if generated:
        workload = test["workload"]
        segment = next(s for s in workload["inline"]["segments"] if s["name"] == "tpcds")
        segment["name"] = "tpcds-generated"
        segment["workload"]["query_stream"] = 0
        workload["inline"]["segments"] = [segment]
        patch["workload"] = workload
    r = c.request(BASE+"/tests/"+test["id"], "PATCH", patch)
    save("test.json", r)
    if r["http_status"] != 200 or not r["body"]["validation"]["fits"]:
        raise RuntimeError("prepared test did not validate")
    print("prepared", test["id"], flush=True)


def launch():
    if (DEST/"launch.json").exists():
        raise RuntimeError("launch recorded; use --watch")
    test = json.loads((DEST/"test.json").read_text())["body"]
    intent = DEST/"launch-intent.json"
    if not intent.exists():
        save("launch-intent.json", {"key": str(uuid.uuid4()), "test_id": test["id"]})
    plan = json.loads(intent.read_text())
    r = Client().request(BASE+"/tests/"+plan["test_id"]+":launch", "POST", {}, idempotency_key=plan["key"])
    save("launch-response.json", r)
    if r["http_status"] != 201:
        raise RuntimeError(f"launch HTTP {r['http_status']}: {r['body'].get('detail')}")
    save("launch.json", r)
    print("analytical run", r["body"]["id"], flush=True)


def watch():
    rid = json.loads((DEST/"launch.json").read_text())["body"]["id"]
    last = None
    observations = json.loads((DEST/"observations.json").read_text()) if (DEST/"observations.json").exists() else []
    deadline = time.monotonic()+3600
    while time.monotonic() < deadline:
        try:
            r = Client().request(BASE+"/runs/"+rid, timeout=30)
        except OSError:
            time.sleep(5)
            continue
        if r["http_status"] != 200:
            time.sleep(5)
            continue
        b = r["body"]
        state = (b["status"], b["phase"], b.get("status_reason"))
        if state != last:
            print(state, flush=True)
            observations.append({"at": datetime.now(timezone.utc).isoformat(), "state": state})
            save("observations.json", observations)
            last = state
        if b["status"] in TERMINAL:
            save("final.json", r)
            print("segments", json.dumps(b.get("result", {}).get("segments", [])), flush=True)
            return 0 if b["status"] == "completed" else 1
        time.sleep(5)
    raise TimeoutError("run still active; observe saved ID instead of relaunching")


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--launch", action="store_true")
    p.add_argument("--watch", action="store_true")
    p.add_argument("--prepare-hub", action="store_true")
    p.add_argument("--prepare-mirror", action="store_true")
    p.add_argument("--prepare-generated", action="store_true")
    p.add_argument("--campaign", default="analytical-20260927")
    a = p.parse_args()
    if not a.campaign.replace("-", "").isalnum(): p.error("invalid campaign")
    DEST = ROOT / "pipelines/live/tests/platform/server/resilience" / a.campaign
    if sum([a.prepare_hub, a.prepare_mirror, a.prepare_generated]) > 1: p.error("choose one preparation")
    if not (a.launch or a.watch or a.prepare_hub or a.prepare_mirror or a.prepare_generated): p.error("choose an action")
    if a.prepare_hub or a.prepare_mirror or a.prepare_generated:
        prepare_hub(mirror=a.prepare_mirror or a.prepare_generated, generated=a.prepare_generated)
    if a.launch: launch()
    if a.watch: raise SystemExit(watch())
