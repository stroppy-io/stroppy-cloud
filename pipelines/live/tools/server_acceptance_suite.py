#!/usr/bin/env python3
"""A two-cell real suite (success + SQL failure), then a full immutable rerun.

Uses only Stroppy Cloud HTTP. --start creates resources; --watch resumes
observation from saved evidence and never repeats a recorded launch.
"""
import argparse
import json
import time
from urllib.error import URLError
from server_api import Client
from server_live_acceptance import BASE, DEST, TERMINAL, save

PG_TEST = "72872cd7-c89e-448b-a77b-c206b1dc40c9"
NOOP_TEST = "0bae3342-5c1c-4c4e-9105-decf257caa68"


def request(path, method="GET", data=None, name=None):
    r = Client().request(BASE+path, method, data, evidence=DEST/name if name else None)
    if r["http_status"] not in {200,201,202,204}:
        raise RuntimeError(f"{method} {path}: HTTP {r['http_status']} {r['body']}")
    return r["body"]


def start():
    if (DEST/"suite-launch.json").exists():
        raise RuntimeError("suite already launched; use --watch")
    tests=[]
    for kind, source in [("success",NOOP_TEST),("sql-failure",PG_TEST)]:
        clone=request(f"/tests/{source}:clone", "POST", {"name":"server-acceptance-"+kind}, kind+"-test-created.json")
        workload=clone["workload"]
        if "inline" not in workload:
            raise RuntimeError("acceptance source must have inline workload")
        segment=workload["inline"]["segments"][0]
        segment["run"]["duration"]="20s"
        segment["log_level"]="warn"
        if kind=="sql-failure":
            segment["workload"]["sql_body"]="SELECT 1 / 0;"
            segment["thresholds"]={"error_rate":0}
        request("/tests/"+clone["id"],"PATCH",{"workload":workload,"keep":"0s"},kind+"-test.json")
        tests.append({"ref":{"id":clone["id"]}})
    suite=request("/suites","POST",{"name":"server acceptance mixed outcome","tests":tests,"concurrency":2},"suite-definition.json")
    launched=request("/suites/"+suite["id"]+":launch","POST",name="suite-launch.json")
    print("suite launched",launched["id"],flush=True)


def wait(path, name):
    deadline=time.monotonic()+1800
    previous=None
    while time.monotonic()<deadline:
        try:
            b=request(path)
        except (OSError,URLError):
            time.sleep(5)
            continue
        state=(b["status"],b.get("phase"),json.dumps(b.get("progress"),sort_keys=True))
        if state!=previous:print(name,*state,flush=True);previous=state
        if b["status"] in TERMINAL:
            return request(path,name=name+"-run.json")
        time.sleep(5)
    raise TimeoutError(f"{name}: inspect the existing run through the server")


def watch():
    sid=json.loads((DEST/"suite-launch.json").read_text())["body"]["id"]
    suite=wait("/suite-runs/"+sid,"suite")
    cells=suite["cells"]
    if len(cells)!=2 or sorted(c["status"] for c in cells)!=["completed","failed"]:
        raise RuntimeError("expected one completed and one failed cell; inspect suite-run.json")
    for cell in cells:
        request("/runs/"+cell["run"]["id"],name="suite-cell-"+cell["status"]+".json")
    source=json.loads((DEST/"suite-cell-failed.json").read_text())["body"]
    if not source.get("result",{}).get("segments"):
        raise RuntimeError("failed run lost partial result")
    rerun_path=DEST/"rerun-launch.json"
    if rerun_path.exists():
        launched=json.loads(rerun_path.read_text())["body"]
    else:
        launched=request("/runs/"+source["id"]+":rerun","POST",{"name":"server acceptance full rerun","keep":"0s"},"rerun-launch.json")
    final=wait("/runs/"+launched["id"],"rerun")
    verified=(final["id"]!=source["id"] and final["graphene"]["run_ref"]!=source["graphene"]["run_ref"]
              and final["status"]=="failed" and final["snapshot"]==source["snapshot"]
              and bool(final.get("result",{}).get("segments")))
    save("suite-rerun-verification.json",{"verified":verified,"suite_id":sid,"source_run":source["id"],"rerun_id":final["id"],
         "expected":"one successful cell, one SQL failure with partial result; rerun repeats the same failing snapshot under a new Graphene run"})
    if not verified:raise RuntimeError("rerun differs from expected immutable snapshot/outcome")


if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--start",action="store_true")
    p.add_argument("--watch",action="store_true")
    args=p.parse_args()
    if not (args.start or args.watch):p.error("choose --start or --watch")
    if args.start:start()
    if args.watch:watch()
