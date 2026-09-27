#!/usr/bin/env python3
"""Operate acceptance runs through Stroppy Cloud; restart touches only local Compose."""
import argparse
from datetime import datetime,timezone
import json
from pathlib import Path
import subprocess
import time
from urllib.error import URLError
from server_api import Client

ROOT=Path(__file__).resolve().parents[3]
DEST=ROOT/"pipelines/live/tests/platform/server/acceptance"
TENANT="server-live-20260924"
BASE=f"/api/v1/t/{TENANT}"
TERMINAL={"completed","failed","cancelled"}


def save(name,value):
    DEST.mkdir(parents=True,exist_ok=True)
    (DEST/name).write_text(json.dumps(value,indent=2)+"\n")


def watch(args):
    c=Client();path=BASE+"/runs/"+args.run_id
    deadline=time.monotonic()+1200
    acted=False
    before=None
    last=None
    while time.monotonic()<deadline:
        try:
            response=c.request(path)
        except (OSError,URLError):
            print(args.name,"API temporarily unavailable; retrying read",flush=True)
            time.sleep(5)
            continue
        if response["http_status"] in {502,503,504}:
            time.sleep(5)
            continue
        if response["http_status"]!=200: raise RuntimeError(f"run read HTTP {response['http_status']}")
        r=response["body"]
        state=(r["status"],r.get("phase"))
        if state!=last: print(args.name,*state,flush=True);last=state
        if not acted and r.get("phase")=="workload" and r["status"]=="running":
            before={"id":r["id"],"graphene":r["graphene"]}
            if args.action=="restart":
                subprocess.run(["docker","compose","--env-file",".env.compose","restart","server"],cwd=ROOT,check=True,stdout=subprocess.DEVNULL)
                for _ in range(30):
                    try:
                        if c.request("/api/v1/public/health")["http_status"]==200:break
                    except OSError:pass
                    time.sleep(1)
                print("local server restarted",flush=True)
            elif args.action=="cancel":
                response=c.request(path+":cancel","POST",evidence=DEST/(args.name+"-cancel.json"))
                if response["http_status"]!=200:raise RuntimeError("cancel rejected")
            acted=True
        if r["status"] in TERMINAL:
            save(args.name+"-run.json",response)
            ok=acted and before=={"id":r["id"],"graphene":r["graphene"]}
            if args.action=="restart":ok=ok and r["status"]=="completed"
            if args.action=="cancel":ok=ok and r["status"]=="cancelled"
            save(args.name+"-lifecycle.json",{"observed_at":datetime.now(timezone.utc).isoformat(),"action":args.action,"action_performed":acted,"run_id":r["id"],"status":r["status"],"same_graphene_run":before=={"id":r["id"],"graphene":r["graphene"]},"verified":ok})
            return 0 if ok else 1
        time.sleep(5)
    raise TimeoutError("run did not finish within 20 minutes; inspect through server")


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--run-id",required=True)
    p.add_argument("--name",required=True)
    p.add_argument("--action",choices=["restart","cancel"],required=True)
    return watch(p.parse_args())

if __name__=="__main__":raise SystemExit(main())
