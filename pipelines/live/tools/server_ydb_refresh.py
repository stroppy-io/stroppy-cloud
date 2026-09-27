#!/usr/bin/env python3
"""Prepare a bounded IAM rotation test through Stroppy Cloud; launch is separate."""
import argparse
import json
from pathlib import Path

from server_api import Client


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--evidence-dir", required=True)
    p.add_argument("--image", required=True)
    p.add_argument("--tenant", default="server-live-20260924")
    p.add_argument("--source-test", default="11374fec-dad3-4406-b14e-1e88b187d29a")
    p.add_argument("--update-existing", action="store_true")
    p.add_argument("--rcu-limit", type=int, default=1000)
    a = p.parse_args()
    dest = Path(a.evidence_dir)
    dest.mkdir(parents=True, exist_ok=True)
    if (dest / "test-created.json").exists() and not a.update_existing:
        raise RuntimeError("draft already exists; inspect its saved identity")
    c, base = Client(), "/api/v1/t/" + a.tenant

    def call(name, path, method="GET", data=None):
        r = c.request(base + path, method, data, evidence=dest / (name + ".json"))
        assert r["http_status"] in {200, 201}, (name, r["http_status"])
        return r["body"]

    database = {"kind": "ydb_managed", "version": "managed",
                "schema": {"id": "db.ydb_managed.params", "version": "1"},
                "params": {"type": "serverless", "location_id": "ru-central1",
                           "throttling_rcu_limit": a.rcu_limit, "provisioned_rcu_limit": 0,
                           "storage_size_limit_gb": 1, "deletion_protection": False}}
    preview = call("database-preview", "/databases:preview", "POST", {"name": "IAM refresh serverless", **database})
    assert not preview["validation"].get("errors")
    source = call("source-test", "/tests/" + a.source_test)
    workload = {"schema": {"id": "workload.stroppy", "version": "1"}, "protocol": "ydb_grpcs",
                "stroppy_version": "6.0.0", "segments": [
                    {"name": "iam-refresh", "workload": {"script": "execute_sql", "sql_body": "SELECT 1;"},
                     "timeout": "20m", "log_level": "info", "thresholds": {"error_rate": 0},
                     "run": {"executor": "constant-vus", "vus": 1, "duration": "15m", "query_timeout": "10s"}}
                ]}
    execution = {"workload": {"stroppy_image": a.image}, "containers": [
        {"name": "runner-1-node-exporter", "set": {
            "image": "mirror.gcr.io/prom/node-exporter@sha256:1b4e4438faca4dd7e001dd445d161a4a2091b0fededa84093b3a8dfeae1f1be0"}}
    ]}
    test = (json.loads((dest / "test-created.json").read_text())["body"] if a.update_existing else
            call("test-created", "/tests/" + a.source_test + ":clone", "POST",
                 {"name": "server managed YDB IAM rotation " + dest.name}))
    test = call("test", "/tests/" + test["id"], "PATCH",
                {"database": {"inline": database}, "workload": {"inline": workload},
                 "sizes": {"runner": {"size": "S"}}, "execution": execution,
                 "provider_profile_id": source["provider_profile_id"], "keep": "0s"})
    assert test["validation"]["fits"], "draft invalid; inspect evidence"
    print("prepared", test["id"], flush=True)


if __name__ == "__main__":
    main()
