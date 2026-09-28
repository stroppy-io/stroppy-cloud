#!/usr/bin/env python3
"""Observe IAM renewal via server only; distinguish replacement from issuer no-op."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import subprocess
import sys
import time
from urllib.parse import urlencode, quote
from urllib.request import Request, urlopen

from record_20260927_checks import ROOT, load, update
from server_api import Client

CASE = "ydb_managed/managed/serverless/execute-sql/server-iam-refresh-15m"


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--campaign", default="ydb-iam-refresh-20260927")
    p.add_argument("--tenant", default="server-live-20260924")
    p.add_argument("--auth-mode", choices=["token-file", "sdk-key"], default="token-file")
    a = p.parse_args()
    dest = ROOT / "tests/platform/server/resilience" / a.campaign
    rid = load(dest / "launch.json")["body"]["id"]
    c, base = Client(), f"/api/v1/t/{a.tenant}/runs/{rid}"

    def save(name, value):
        (dest / (name + ".json")).write_text(json.dumps(value, indent=2) + "\n")

    def tool(name, args):
        r = subprocess.run([sys.executable, str(ROOT / "tools" / (name + ".py")), *args],
                           text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        (dest / (name + ".log")).write_text(r.stdout)
        assert r.returncode == 0, name + " failed; inspect evidence"
        print(name, "passed", flush=True)

    deadline, last = time.monotonic() + 3600, None
    while time.monotonic() < deadline:
        r = c.request(base)
        assert r["http_status"] == 200
        run = r["body"]
        state = (run["status"], run["phase"])
        if state != last:
            print(datetime.now(timezone.utc).isoformat(), state, flush=True)
            last = state
        if run["phase"] == "workload" and not (dest / "managed-ready.json").exists():
            t = c.request(base + "/tree")
            nodes = []
            def visit(n):
                if "DatabaseServerless" in n.get("kind", ""):
                    nodes.append({"ref": n["ref"], "phase": n.get("phase")})
                for child in n.get("children", []):
                    visit(child)
            if t["http_status"] == 200:
                visit(t["body"])
                if nodes and all(n["phase"] == "ready" for n in nodes):
                    save("managed-ready", {"run_id": rid, "observed_at": r["observed_at"],
                                           "verified": True, "nodes": nodes, "tree": t["body"]})
        save("current", r)
        if run["status"] in {"completed", "failed", "cancelled"}:
            save("final", r)
            break
        time.sleep(20)
    else:
        raise TimeoutError("observe saved run ID; do not relaunch")
    common = ["--tenant", a.tenant, "--run-id", rid]
    tool("record_server_case", common + ["--case", CASE, "--workload", "execute/sql", "--refresh"])
    tool("verify_server_cleanup", common + ["--evidence", str(dest / "cleanup.json")])
    assert run["status"] == "completed", "workload failed; inspect final.json"
    segment = run["result"]["segments"][0]
    assert segment["status"] == "completed" and segment["metrics"]["failed_queries_total"]["value"] == 0
    events = []
    expected_ns = "t-" + a.tenant
    for message in ("YDB IAM token refreshed", "YDB authentication token loaded from file",
                    "Using Yandex Cloud service account key authentication"):
        result = c.request(base + "/logs?" + urlencode({"q": message, "limit": 100, "direction": "newer"}))
        assert result["http_status"] == 200
        for row in result["body"].get("data", []):
            if message not in row.get("message", ""):
                continue
            fields = row.get("fields", {})
            assert fields.get("graphene.run") == rid and fields.get("graphene.namespace") == expected_ns
            if fields.get("graphene.activity") != "stroppy.segment.run":
                continue
            # Preserve event identity, not arbitrary SDK log payloads/credentials.
            events.append({"message": message, "time": row["time"],
                           "activity": fields.get("graphene.activity"), "machine": row.get("machine"),
                           **{key: fields[key] for key in ("credential_state", "previous_expires_at", "expires_at") if key in fields}})
    save("renewal-events", {"run_id": rid, "events": events})
    refresh = [e for e in events if e["message"] == "YDB IAM token refreshed"]
    loaded = [e for e in events if e["message"] == "YDB authentication token loaded from file"]
    auth_name = "sdk-auth" if a.auth_mode == "sdk-key" else "rotation"
    if a.auth_mode == "sdk-key":
        active = [e for e in events if e["message"] == "Using Yandex Cloud service account key authentication"]
        assert active, "missing SDK service-account authentication evidence"
        assert not refresh and not loaded, "legacy token-file authentication still active"
        window_start = segment["started_at"]
        replacements = []
    else:
        assert refresh, "missing real IAM renewal response"
        assert any(e["time"] < refresh[0]["time"] for e in loaded), "no initial workload token load"
        replacements = []
        for event in refresh:
            if event.get("credential_state") == "unchanged":
                assert event["expires_at"] == event["previous_expires_at"], "unchanged token has inconsistent expiry"
                assert datetime.fromisoformat(event["expires_at"].replace("Z", "+00:00")) > datetime.fromisoformat(event["time"].replace("Z", "+00:00")), "issuer returned expired token"
            else:
                # Legacy logs have no state. Require actual reload for those too.
                assert any(e["time"] >= event["time"] for e in loaded), "no Stroppy reload after replacement"
                replacements.append(event)
        assert segment["finished_at"] > refresh[0]["time"], "segment did not continue after refresh"
        window_start = refresh[0]["time"]
    after_refresh = c.request(base + "/metrics:raw", "POST", {
        "query": '{__name__="stroppy_run_query_operations_total","stroppy.segment"="iam-refresh"}',
        "start": window_start, "end": segment["finished_at"], "step": "5s"})
    assert after_refresh["http_status"] == 200
    counters = after_refresh["body"].get("data", {}).get("result", [])
    advancing = []
    for series in counters:
        labels, values = series.get("metric", {}), series.get("values", [])
        assert labels.get("graphene.run") == rid and labels.get("graphene.namespace") == expected_ns
        if len(values) >= 2 and float(values[-1][1]) > float(values[0][1]):
            advancing.append({"labels": labels, "first": values[0], "last": values[-1]})
    assert advancing, "no continuing workload queries observed in authentication window"
    ready = load(dest / "managed-ready.json")
    assert ready["run_id"] == rid and ready["verified"]
    artifact_listing = c.request(base + "/artifacts")
    assert artifact_listing["http_status"] == 200
    configs = []
    for artifact in artifact_listing["body"]["data"]:
        if "config" not in str(artifact).lower():
            continue
        req = Request(c.base + base + "/artifacts/" + quote(artifact["id"], safe=""),
                      headers={"Authorization": "Bearer " + c.token})
        with urlopen(req, timeout=120) as response:
            raw = response.read(2 * 1024 * 1024 + 1)
        assert len(raw) <= 2 * 1024 * 1024
        config = json.loads(raw)
        def inspect(v):
            if isinstance(v, dict):
                for key, value in v.items():
                    assert key not in {"authToken", "authTokenFile", "serviceAccountKeyFile", "private_key", "privateKey", "sa_key_json"} or not value, "runtime auth leaked to config artifact"
                    inspect(value)
            elif isinstance(v, list):
                for value in v: inspect(value)
        inspect(config)
        configs.append(artifact["id"])
    assert configs, "no public config inspected"
    scope = ("Official SDK service-account key authentication and continuous queries; token reissuance tested locally with short-lived IAM credentials, not claimed for this live run."
             if a.auth_mode == "sdk-key" else "Real YC renewal response and continuing queries; rotation_verified distinguishes replacement from unchanged credentials.")
    save(auth_name, {"run_id": rid, "verified": True, "auth_mode": a.auth_mode,
                      "renewal_verified": True if a.auth_mode == "token-file" else None, "rotation_verified": bool(replacements), "events": events,
                      "segment": segment,
                      ("queries_during_segment" if a.auth_mode == "sdk-key" else "queries_after_refresh"): advancing,
                      "public_configs_without_token": configs,
                      "scope": scope})
    for name, output in [("verify_server_artifacts", "artifacts"), ("verify_server_telemetry", "telemetry")]:
        tool(name, common + ["--evidence", str(dest / (output + ".json"))])
    finished = datetime.fromisoformat(run["finished_at"].replace("Z", "+00:00"))
    while (datetime.now(timezone.utc) - finished).total_seconds() < 120:
        time.sleep(15)
    tool("verify_server_cpu_tail", common + ["--evidence", str(dest / "cpu-tail.json")])
    changes = {
        "smoke": ("passed", scope + " No query errors.", dest / (auth_name + ".json"), "/verified"),
        "infrastructure_provisioning": ("passed", "Managed serverless resource explicitly ready during workload; runner executed the segment.", dest / "managed-ready.json", "/verified"),
        "replication": ("not_applicable", "Managed serverless service; internal replication is outside this test.", None, None),
        "native_tps": ("not_applicable", "execute_sql reports queries/s, not logical transaction TPS.", None, None),
        "managed_database_metrics": ("not_run", "This scenario verifies authentication renewal, not managed database telemetry.", None, None),
    }
    for key, name, pointer in [
        ("native_otlp_metrics", "telemetry", "/native_verified"),
        ("component_logs", "telemetry", "/logs_verified"),
        ("component_metrics", "cpu-tail", "/verified"),
        ("artifacts", "artifacts", "/verified"),
        ("cleanup", "cleanup", "/verified"),
    ]:
        changes[key] = ("passed", "Verified through server API; see scoped evidence.", dest / (name + ".json"), pointer)
    update(ROOT / "tests" / CASE, rid, changes)
    tool("live", ["progress"])
    tool("live", ["validate"])
    print("YDB authentication verified:", a.auth_mode, "credential replacement:", bool(replacements), rid, flush=True)


if __name__ == "__main__":
    main()
