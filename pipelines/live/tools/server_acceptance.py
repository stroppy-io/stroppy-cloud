#!/usr/bin/env python3
"""Run local server acceptance and retain assertion outcomes, not sensitive logs."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

ROOT = Path(__file__).resolve().parents[3]
DEST = ROOT / "pipelines/live/tests/platform/server/acceptance"


def source_digest():
    digest = hashlib.sha256()
    for folder in ("cmd", "internal", "openapi/parts", "pipelines/spec", "pipelines/schemas", "pipelines/internal", "pipelines/stroppycfg", "web/src/schemas"):
        for p in sorted((ROOT/folder).rglob("*")):
            if p.is_file() and p.suffix in {".go", ".json", ".yaml", ".ts"}:
                digest.update(str(p.relative_to(ROOT)).encode()+b"\0"+p.read_bytes())
    for name in ("go.mod", "go.sum", "pipelines/go.mod", "pipelines/go.sum"):
        digest.update((ROOT/name).read_bytes())
    return digest.hexdigest()


def run(args):
    DEST.mkdir(parents=True,exist_ok=True)
    local = ROOT/".local/server-acceptance"
    local.mkdir(parents=True,exist_ok=True)
    log = local/(args.name+".jsonl")
    command = ["go", "test", "-json", "-tags=integration", "./cmd/stroppy-cloud/application", "-count=1", "-p=1", "-timeout=20m", "-run", args.run]
    if args.race:command.insert(2,"-race")
    before = source_digest()
    started = datetime.now(timezone.utc).isoformat()
    begin = time.monotonic()
    env = dict(os.environ,STROPPY_ACCEPTANCE_REPORT="1")
    cases, operations = {}, {}
    with open(log,"w",encoding="utf8") as out:
        os.chmod(log,0o600)
        proc = subprocess.Popen(command,cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
        for line in proc.stdout:
            out.write(line)
            try: event=json.loads(line)
            except ValueError: continue
            name=event.get("Test")
            if name and event.get("Action") in {"pass","fail","skip"}:
                cases[name]={"status":event["Action"],"seconds":event.get("Elapsed",0)}
                if "/" not in name: print(name,event["Action"],flush=True)
            output=event.get("Output","")
            if "ACCEPTANCE_HTTP " in output:
                operations[name]=json.loads(output.split("ACCEPTANCE_HTTP ",1)[1])
        code=proc.wait()
    after=source_digest()
    report={"started_at":started,"finished_at":datetime.now(timezone.utc).isoformat(),"seconds":round(time.monotonic()-begin,3),"command":command,"git_head":subprocess.check_output(["git","rev-parse","HEAD"],cwd=ROOT,text=True).strip(),"source_sha256_before":before,"source_sha256_after":after,"source_changed_during_run":before!=after,"exit_code":code,"tests":cases,"http_observations":operations,"note":"HTTP observations prove route exercise only. Semantic assertions are linked in capabilities.json. Graphene and cloud activities are simulated; PostgreSQL and HTTP are real."}
    (DEST/(args.name+".json")).write_text(json.dumps(report,indent=2)+"\n")
    print(json.dumps({"exit_code":code,"tests":len(cases),"failed":[k for k,v in cases.items() if v["status"]=="fail"],"seconds":report["seconds"],"private_log":str(log)}))
    return code


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--name",default="local")
    parser.add_argument("--run",default="Test")
    parser.add_argument("--race",action="store_true")
    args=parser.parse_args()
    if not args.name.replace("-","").replace("_","").isalnum(): parser.error("invalid report name")
    return run(args)

if __name__=="__main__":
    raise SystemExit(main())
