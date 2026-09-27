#!/usr/bin/env python3
"""Check anonymous metrics of a temporary share, then revoke it (never save its token)."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import urlopen
from server_api import Client


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--tenant",required=True)
    p.add_argument("--run-id",required=True)
    p.add_argument("--evidence",required=True)
    a=p.parse_args();c=Client();base=f"/api/v1/t/{a.tenant}"
    created=c.request(base+"/runs/"+a.run_id+":share","POST",{"scope":"metrics"})
    if created["http_status"]!=201:raise RuntimeError("share creation failed")
    sh=created["body"];path="/api/v1/public/share/"+sh["token"]+"/metrics"
    report={"observed_at":datetime.now(timezone.utc).isoformat(),"run_id":a.run_id}
    try:
        with urlopen(c.base+path,timeout=120) as response:
            body=json.load(response)
            report.update(http_status=response.status,series_count=len(body["series"]),metric_errors=body.get("errors",[]),
                          has_points=any(s["points"] for s in body["series"]))
    finally:
        revoked=c.request(base+"/shares/"+sh["id"],"DELETE")
        report["revoke_status"]=revoked["http_status"]
    try:
        with urlopen(c.base+path,timeout=120) as response:report["revoked_read_status"]=response.status
    except HTTPError as error:report["revoked_read_status"]=error.code
    report["verified"]=(report.get("http_status")==200 and report.get("has_points") and not report.get("metric_errors")
                        and report["revoke_status"]==204 and report["revoked_read_status"] in {404,410})
    dest=Path(a.evidence);dest.parent.mkdir(parents=True,exist_ok=True)
    dest.write_text(json.dumps(report,indent=2)+"\n");print(json.dumps(report,indent=2))
    return 0 if report["verified"] else 1


if __name__=="__main__":raise SystemExit(main())
