#!/usr/bin/env python3
"""Verify Graphene's resource-tree projection through the server after teardown."""
import argparse
from datetime import datetime,timezone
import json
from pathlib import Path
from server_api import Client


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--tenant",required=True)
    p.add_argument("--run-id",required=True)
    p.add_argument("--evidence",required=True)
    a=p.parse_args();c=Client();base=f"/api/v1/t/{a.tenant}/runs/{a.run_id}"
    run=c.request(base);tree=c.request(base+"/tree")
    if run["http_status"]!=200 or tree["http_status"]!=200:raise RuntimeError("cannot read cleanup state")
    nodes=[]
    def visit(n):
        if n["kind"] not in {"run","artifact"}:nodes.append({"ref":n["ref"],"phase":n.get("phase")})
        for child in n.get("children",[]):visit(child)
    visit(tree["body"])
    active=[n for n in nodes if n["phase"]!="deleted"]
    b=run["body"]
    report={"observed_at":datetime.now(timezone.utc).isoformat(),"run_id":a.run_id,"run_status":b["status"],
            "stand_kept":b["stand_kept"],"remaining_infrastructure":active,"tree":tree["body"],
            "verified":b["status"] in {"completed","failed","cancelled"} and not b["stand_kept"] and not active,
            "scope":"Server/Graphene resource-tree assertion; no direct YC or Kubernetes query."}
    dest=Path(a.evidence);dest.parent.mkdir(parents=True,exist_ok=True)
    dest.write_text(json.dumps(report,indent=2)+"\n")
    print(json.dumps({k:v for k,v in report.items() if k!="tree"},indent=2))
    return 0 if report["verified"] else 1


if __name__=="__main__":raise SystemExit(main())
