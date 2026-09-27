#!/usr/bin/env python3
"""Prepare and launch server-only three-member MySQL/Galera checks.

Saved launch intents make HTTP retries use the same idempotency key.
SQL assertions fail the workload if the cluster loses its expected membership.
They verify the serving member; per-member replication still needs telemetry.
"""
import argparse
import json
from pathlib import Path
import uuid

from server_api import Client
from server_live_acceptance import ROOT, BASE

DEST = ROOT / "pipelines/live/tests/platform/server/resilience/ha-20260927"
SOURCE = "5d32a0d6-0aa5-4e95-ac10-cc41e1c4b1c7"
NODE_IMAGE = "docker.io/prom/node-exporter@sha256:1b4e4438faca4dd7e001dd445d161a4a2091b0fededa84093b3a8dfeae1f1be0"


def request(kind, name, path, method="GET", data=None, key=None):
    dest = DEST / kind
    dest.mkdir(parents=True, exist_ok=True)
    r = Client().request(BASE + path, method, data, evidence=dest/(name+".json"), idempotency_key=key)
    if r["http_status"] not in {200, 201, 202}:
        raise RuntimeError(f"{path}: HTTP {r['http_status']} {r['body'].get('detail')}")
    return r["body"]


def prepare(kind):
    dest = DEST / kind
    if (dest/"test-created.json").exists():
        raise RuntimeError("test already created: inspect saved evidence before modifying")
    maria = kind == "mariadb"
    version = "11.8" if maria else "8.4"
    predicate = (
        "(SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_CLUSTER_SIZE')='3' "
        "AND (SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_LOCAL_STATE')='4' "
        "AND (SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_READY')='ON' "
        "AND (SELECT VARIABLE_VALUE FROM information_schema.GLOBAL_STATUS WHERE VARIABLE_NAME='WSREP_CLUSTER_STATUS')='Primary'"
        if maria else
        "(SELECT COUNT(*)=3 AND SUM(MEMBER_STATE='ONLINE')=3 AND SUM(MEMBER_ROLE='PRIMARY')=1 "
        "FROM performance_schema.replication_group_members)"
    )
    init = """USE stroppy;
CREATE TABLE live_cluster_probe (id INT PRIMARY KEY, writes BIGINT NOT NULL);
DELIMITER //
CREATE PROCEDURE live_assert_cluster()
BEGIN
  IF NOT COALESCE((%s), 0) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='live acceptance: expected three healthy cluster members';
  END IF;
  INSERT INTO live_cluster_probe VALUES (1,1) ON DUPLICATE KEY UPDATE writes=writes+1;
END//
DELIMITER ;
""" % predicate
    params = {"version": version, "replication": "galera" if maria else "group", "proxysql": 1, "init_sql": init}
    params.update({"replicas": 0, "galera_nodes": 3} if maria else {"replicas": 2, "single_primary": True})
    database = {"kind": kind, "version": version, "params": params, "schema": {"id": "db."+kind+".params", "version": "1"}}
    preview = request(kind, "database-preview", "/databases:preview", "POST", {"name": "live HA "+kind, **database})
    if preview["validation"].get("errors"):
        raise RuntimeError("invalid database preview")
    test = request(kind, "test-created", "/tests/"+SOURCE+":clone", "POST", {"name": "server HA "+kind+" 3 members proxysql"})
    machines = (["db-1", "db-2", "db-3"] if maria else ["db-1", "db-replica-1", "db-replica-2"]) + ["proxy-1", "runner-1"]
    execution = test["execution"]
    execution["containers"] = [{"name": n+"-node-exporter", "set": {"image": NODE_IMAGE}} for n in machines]
    workload = {"schema": {"id": "workload.stroppy", "version": "1"}, "protocol": "mysql", "stroppy_version": "6.0.0",
                "segments": [{"name": "cluster-health-writes", "workload": {"script": "execute_sql", "sql_body": "CALL live_assert_cluster();"},
                              "run": {"executor": "constant-vus", "vus": 1, "duration": "2m", "query_timeout": "10s"},
                              "thresholds": {"error_rate": 0}, "log_level": "info", "timeout": "5m"}]}
    sizes = {n["role"]: {"size": "S"} for n in preview["topology_preview"]["nodes"] if not n.get("colocated_with")}
    sizes["runner"] = {"size": "S"}
    test = request(kind, "test", "/tests/"+test["id"], "PATCH", {"database": {"inline": database}, "workload": {"inline": workload},
                   "sizes": sizes, "execution": execution, "keep": "0s"})
    if not test["validation"]["fits"]:
        raise RuntimeError("test did not validate; inspect test.json")
    print(kind, "prepared", test["id"], flush=True)


def launch(kind):
    dest = DEST/kind
    if (dest/"launch.json").exists():
        raise RuntimeError("launch already recorded; observe saved run")
    test = json.loads((dest/"test.json").read_text())["body"]
    intent = dest/"launch-intent.json"
    if not intent.exists():
        intent.write_text(json.dumps({"test_id": test["id"], "key": str(uuid.uuid4())}, indent=2)+"\n")
    plan = json.loads(intent.read_text())
    run = request(kind, "launch-response", "/tests/"+plan["test_id"]+":launch", "POST", {}, key=plan["key"])
    (dest/"launch.json").write_bytes((dest/"launch-response.json").read_bytes())
    print(kind, "launched", run["id"], flush=True)


def prepare_mirror(kind):
    if (DEST/kind/"test-created.json").exists():
        raise RuntimeError("test already prepared")
    old = ROOT/"pipelines/live/tests/platform/server/resilience/ha-20260927"/kind
    source = json.loads((old/"test.json").read_text())["body"]
    test = request(kind, "test-created", "/tests/"+source["id"]+":clone", "POST", {"name": source["name"]+" mirror"})
    rid = json.loads((old/"launch.json").read_text())["body"]["id"]
    run = request(kind, "source-run", "/runs/"+rid)
    digests = {
        "mysql:8.4": "library/mysql@sha256:0744ee5ef89ce6ccfa13de3e579fe6b9e27f93dd70da9c06d2c908b1b193fb8d",
        "mariadb:11.8": "library/mariadb@sha256:79d59758afc91b89b120b0a8904d637f5a3b3e1c4900f29b740d6d46c72fef68",
        "prom/mysqld-exporter:v0.19.0": "prom/mysqld-exporter@sha256:eacb4b18e2ec1e0abdf2d64851b68526c964f6d9cb3e9458fb5d5f5062ea94c1",
        "proxysql/proxysql:2.7.3": "proxysql/proxysql@sha256:a4d6c35c29498b294d771898f747e2143bc5c8f8b338a58324b1152144f970bb",
        NODE_IMAGE: NODE_IMAGE.removeprefix("docker.io/"),
    }
    execution = test["execution"]
    execution["containers"] = [{"name": c["name"], "set": {"image": "mirror.gcr.io/"+digests[c["image"]]}}
                               for c in run["run_spec"]["values"]["containers"]]
    test = request(kind, "test", "/tests/"+test["id"], "PATCH", {"execution": execution})
    if not test["validation"]["fits"]:
        raise RuntimeError("mirror test invalid")
    print(kind, "mirror prepared", test["id"], flush=True)


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("kind", choices=["mysql", "mariadb"])
    p.add_argument("--prepare", action="store_true")
    p.add_argument("--launch", action="store_true")
    p.add_argument("--prepare-mirror", action="store_true")
    p.add_argument("--campaign", default="ha-20260927")
    a = p.parse_args()
    if not a.campaign.replace("-", "").isalnum(): p.error("invalid campaign")
    DEST = ROOT / "pipelines/live/tests/platform/server/resilience" / a.campaign
    if a.prepare and a.prepare_mirror: p.error("choose one preparation")
    if not (a.prepare or a.prepare_mirror or a.launch): p.error("choose an action")
    if a.prepare: prepare(a.kind)
    if a.prepare_mirror: prepare_mirror(a.kind)
    if a.launch: launch(a.kind)
