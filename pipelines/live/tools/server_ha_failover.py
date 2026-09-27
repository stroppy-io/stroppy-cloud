#!/usr/bin/env python3
"""Prepare a server test with three VM-local, strictly scoped HA probe containers."""
import argparse
import copy
import json
from pathlib import Path
import re

from server_api import Client


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--source-run", required=True)
    p.add_argument("--source-test", required=True)
    p.add_argument("--probe-image", required=True)
    p.add_argument("--evidence-dir", required=True)
    p.add_argument("--tenant", default="server-live-20260924")
    a = p.parse_args()
    dest = Path(a.evidence_dir)
    dest.mkdir(parents=True, exist_ok=True)
    client, base = Client(), "/api/v1/t/"+a.tenant

    def call(name, path, method="GET", body=None):
        r = client.request(base+path, method, body, evidence=dest/(name+".json"))
        if r["http_status"] not in {200, 201}: raise RuntimeError(f"{name}: HTTP {r['http_status']}; see evidence")
        return r["body"]

    if (dest/"test-created.json").exists():
        raise RuntimeError("test exists; inspect saved identity before editing")
    run = call("source-run", "/runs/"+a.source_run)
    source = call("source-test", "/tests/"+a.source_test)
    compiled = run["run_spec"]["values"]
    kind = run["snapshot"]["database"]["kind"]
    if kind not in {"mysql", "mariadb"}: raise ValueError("currently MySQL family only")
    identity = "@@server_uuid" if kind == "mysql" else "@@hostname"
    db_containers = [c for c in compiled["containers"] if c["kind"] == "database"]
    assert len(db_containers) == 3
    auth = re.fullmatch(r"([^:]+):([^@]*)@tcp\([^)]*\)/stroppy", compiled["workload"]["url"])
    assert auth, "unexpected DSN shape; do not guess credentials"
    database = copy.deepcopy(run["snapshot"]["database"])
    database["params"]["init_sql"] = """USE stroppy;
CREATE TABLE live_ha_markers (id INT PRIMARY KEY, payload VARCHAR(100) NOT NULL, writer VARCHAR(100) NOT NULL);
CREATE TABLE live_ha_control (id INT PRIMARY KEY, done INT NOT NULL);
INSERT INTO live_ha_control VALUES (1,0);
DELIMITER //
CREATE PROCEDURE live_ha_verify()
BEGIN
  IF (SELECT done FROM live_ha_control WHERE id=1) <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='HA failover probe did not complete';
  END IF;
END//
DELIMITER ;
"""
    execution = copy.deepcopy(source["execution"])
    execution["additional_containers"] = []
    hosts = ",".join("${ip:"+c["machine"]+"}" for c in db_containers)
    for c in db_containers:
        name = c["machine"]+"-ha-probe"
        execution["additional_containers"].append({
            "name": name, "role": c["role"], "kind": "addon", "machine": c["machine"], "image": a.probe_image,
            "depends_on": [x["name"] for x in db_containers]+["proxy-1-proxysql"],
            "env": {"DB_KIND": kind, "LOCAL_HOST": "${ip:"+c["machine"]+"}", "DB_HOSTS": hosts,
                    "PROXY_HOST": "${ip:proxy-1}", "TARGET_CONTAINER": c["name"], "PROBE_CONTAINER": name,
                    "DB_USER": "root", "DB_PASSWORD": c["env"]["MYSQL_ROOT_PASSWORD"],
                    "PROXY_USER": auth[1], "PROXY_PASSWORD": auth[2]},
            "mounts": [{"source": "/var/run/docker.sock", "target": "/var/run/docker.sock"}],
            "healthcheck": {"cmd": ["/ha-probe", "health"], "interval": "1s", "retries": 10},
            "restart": "no",
        })
    roles = sorted({c["role"] for c in db_containers})
    existing_flows = {(f["from_role"], f["to_role"], f["port"]) for f in compiled["flows"]}
    execution["flows"] = [{"from_role": role, "to_role": to, "protocol": "tcp", "port": port, "label": "ha-probe"}
                          for role in roles for to, port in [("proxy", 6033)]+[(r, 3306) for r in roles]
                          if (role, to, port) not in existing_flows]
    def segment(name, sql, duration=None):
        return {"name": name, "workload": {"script": "execute_sql", "sql_body": sql},
                "run": {"executor": "constant-vus", "vus": 1, "duration": duration, "query_timeout": "5s"} if duration else
                       {"executor": "shared-iterations", "vus": 1, "iterations": 1, "query_timeout": "5s"},
                "thresholds": {"error_rate": 1 if duration else 0}, "timeout": "8m", "log_level": "info"}
    workload = {"schema": {"id": "workload.stroppy", "version": "1"}, "protocol": "mysql", "stroppy_version": "6.0.0",
                "segments": [segment("start-ha-probe", "INSERT INTO live_ha_markers VALUES (0,'before-fault',"+identity+");"),
                             segment("during-failover", "SELECT 1;", "4m"), segment("verify-ha-probe", "CALL live_ha_verify();")]}
    test = call("test-created", "/tests/"+a.source_test+":clone", "POST", {"name": "server "+kind+" primary outage and exact replica rows"})
    test = call("test", "/tests/"+test["id"], "PATCH", {"database": {"inline": database}, "workload": {"inline": workload},
                                                              "execution": execution, "keep": "0s"})
    assert test["validation"]["fits"], "test did not validate"
    print("prepared", test["id"])


if __name__ == "__main__":
    main()
