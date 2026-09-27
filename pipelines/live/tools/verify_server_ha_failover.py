#!/usr/bin/env python3
"""Verify full HA probe evidence fetched exclusively through Stroppy Cloud."""
import argparse
import json
from pathlib import Path
from urllib.parse import urlencode

from server_api import Client


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--tenant", required=True)
    p.add_argument("--run-id", required=True)
    p.add_argument("--evidence-dir", required=True)
    a = p.parse_args()
    dest = Path(a.evidence_dir)
    dest.mkdir(parents=True, exist_ok=True)
    client, base = Client(), f"/api/v1/t/{a.tenant}/runs/{a.run_id}"
    run = client.request(base, evidence=dest/"ha-run.json")
    assert run["http_status"] == 200
    events, cursor, cursors = [], None, set()
    for page in range(100):
        query = {"q": "ha_event", "limit": 100, "direction": "older"}
        if cursor: query["cursor"] = cursor
        response = client.request(base+"/logs?"+urlencode(query), evidence=dest/f"ha-logs-{page}.json")
        assert response["http_status"] == 200
        for row in response["body"]["data"]:
            assert row["fields"]["graphene.run"] == a.run_id
            message = json.loads(row["message"])
            assert "ha_event" in message
            events.append(message)
        cursor = response["body"].get("older")
        if not cursor: break
        assert cursor not in cursors, "pagination did not advance"
        cursors.add(cursor)
    else:
        raise RuntimeError("probe log page limit exceeded")
    events.sort(key=lambda e: e["at"])
    grouped = {}
    for e in events: grouped.setdefault(e["ha_event"], []).append(e)
    errors = []
    if grouped.get("failed") or grouped.get("emergency_restart"): errors.append("controller failed")
    for name in ["verified", "primary_stopped", "primary_restarted", "proxy_write_recovered"]:
        if len(grouped.get(name, [])) != 1: errors.append("expected one "+name)
    snapshots = grouped.get("replicas_equal", [])
    if [len(e["hosts"]) for e in snapshots] != [3, 2, 3]: errors.append("missing before/during/after replica equality")
    if not errors:
        before, during, after = [e["rows"] for e in snapshots]
        if len(before) != 1 or len(during) != 2 or after != during or during[0] != before[0]: errors.append("marker rows differ")
        if len(before) == 1 and len(after) == 2:
            if before[0]["ID"] != 0 or before[0]["Payload"] != "before-fault": errors.append("initial marker invalid")
            if after[1]["ID"] != 1 or after[1]["Payload"] != "written-while-primary-down": errors.append("outage marker invalid")
            if not before[0]["Writer"] or not after[1]["Writer"] or before[0]["Writer"] == after[1]["Writer"]: errors.append("serving writer unchanged")
        hosts = [set(e["hosts"]) for e in snapshots]
        stopped_host = grouped["primary_stopped"][0]["host"]
        if len(hosts[0]) != 3 or hosts[2] != hosts[0] or hosts[1] != hosts[0]-{stopped_host}: errors.append("replica host sets differ")
        if grouped["primary_stopped"][0]["container_id"] != grouped["primary_restarted"][0]["container_id"]: errors.append("different container restarted")
    body = run["body"]
    gated = any(c.get("env", {}).get("WAIT_FOR_LOAD_GATE") == "1" for c in body["run_spec"]["values"]["containers"])
    if gated and len(grouped.get("load_gate_open", [])) != 3: errors.append("not all controllers observed the workload gate")
    if body["status"] != "completed": errors.append("run not completed")
    segments = body.get("result", {}).get("segments", [])
    if len(segments) != 3 or any(s["status"] != "completed" for s in segments): errors.append("segments incomplete")
    for s in segments:
        if s["name"] in {"start-ha-probe", "verify-ha-probe"} and s["metrics"]["failed_queries_total"]["value"] != 0:
            errors.append("control SQL failed: "+s["name"])
    report = {"run_id": a.run_id, "verified": not errors, "errors": errors, "events": events,
              "scope": "Exact probe-row consistency and proxy write availability across one primary container stop/start. Not a VM/disk outage or losslessness proof for arbitrary workloads."}
    (dest/"ha-verification.json").write_text(json.dumps(report, indent=2)+"\n")
    print(json.dumps({k:v for k,v in report.items() if k != "events"}))
    return 0 if report["verified"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
