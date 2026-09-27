#!/usr/bin/env python3
"""Prepare (never launch) a Patroni/HAProxy failover test through the server."""
import argparse
import json
from pathlib import Path
from urllib.parse import urlparse

from server_api import Client


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--source-test", required=True)
    p.add_argument("--source-run", required=True)
    p.add_argument("--probe-image", required=True)
    p.add_argument("--evidence-dir", required=True)
    p.add_argument("--tenant", default="server-live-20260924")
    p.add_argument("--name", default="server Patroni primary outage and exact replica rows")
    p.add_argument("--update-existing", action="store_true", help="Update the recorded draft; never launch or resume a run.")
    a = p.parse_args()
    dest = Path(a.evidence_dir)
    dest.mkdir(parents=True, exist_ok=True)
    if (dest/"test-created.json").exists() and not a.update_existing: raise RuntimeError("test already prepared")
    c, base = Client(), "/api/v1/t/"+a.tenant

    def call(name, path, method="GET", data=None):
        r = c.request(base+path, method, data, evidence=dest/(name+".json"))
        assert r["http_status"] in {200, 201}, (name, r["http_status"])
        return r["body"]

    source = call("source-test", "/tests/"+a.source_test)
    run = call("source-run", "/runs/"+a.source_run)
    compiled = run["run_spec"]["values"]
    auth = urlparse(compiled["workload"]["url"])
    assert auth.username and auth.password and auth.path == "/postgres"
    dbs = ["db-1", "db-replica-1", "db-replica-2"]
    etcds = ["etcd-1", "etcd-2", "etcd-3"]
    database = {"kind": "postgres", "version": "17", "schema": {"id": "db.postgres.params", "version": "1"},
                "params": {"version": "17", "ha": "patroni", "replicas": 2, "etcd_nodes": 3, "haproxy": 1,
                           "init_sql": """CREATE TABLE live_ha_markers (id INT PRIMARY KEY, payload VARCHAR(100) NOT NULL, writer VARCHAR(100) NOT NULL);
CREATE TABLE live_ha_control (id INT PRIMARY KEY, done INT NOT NULL, started INT NOT NULL);
INSERT INTO live_ha_control VALUES (1,0,0);
CREATE PROCEDURE live_ha_tick() LANGUAGE plpgsql AS $$ BEGIN
  IF (SELECT started FROM live_ha_control WHERE id=1) = 0 THEN
    UPDATE live_ha_control SET started=1 WHERE id=1;
  END IF;
END; $$;
CREATE PROCEDURE live_ha_verify() LANGUAGE plpgsql AS $$ BEGIN
  IF (SELECT done FROM live_ha_control WHERE id=1) <> 1 THEN
    RAISE EXCEPTION 'HA failover probe did not complete';
  END IF;
END; $$;
"""}}
    preview = call("database-preview", "/databases:preview", "POST", {"name": "Patroni HA failover", **database})
    assert not preview["validation"].get("errors")
    sizes = {n["role"]: {"size": "S"} for n in preview["topology_preview"]["nodes"] if not n.get("colocated_with")}
    sizes["runner"] = {"size": "S"}
    node_image = next(x["image"] for x in compiled["containers"] if x["name"] == "db-1-node-exporter")
    exporter_image = next(x["image"] for x in compiled["containers"] if x["name"] == "db-1-postgres-exporter")
    execution = {"workload": source["execution"]["workload"], "containers": [], "additional_containers": [], "flows": []}
    for name in dbs+etcds+["proxy-1", "runner-1"]:
        execution["containers"].append({"name": name+"-node-exporter", "set": {"image": node_image}})
    for name in dbs:
        execution["containers"].extend([
            {"name": name+"-postgres", "set": {"image": "docker.stroppy.io/stroppy-io/patroni@sha256:38091476721967c498b6aed59e530a1ca0771f3206f909cb69ea87b76df6e9db"}},
            {"name": name+"-postgres-exporter", "set": {"image": exporter_image}},
        ])
        execution["additional_containers"].append({
            "name": name+"-ha-probe", "role": "db" if name == "db-1" else "db-replica", "kind": "addon", "machine": name,
            "image": a.probe_image, "restart": "no", "depends_on": [n+"-postgres" for n in dbs]+["proxy-1-haproxy"],
            "env": {"DB_KIND": "postgres", "DB_USER": auth.username, "DB_PASSWORD": auth.password,
                    "WAIT_FOR_LOAD_GATE": "1",
                    "LOCAL_HOST": "${ip:"+name+"}", "DB_HOSTS": ",".join("${ip:"+n+"}" for n in dbs),
                    "PROXY_HOST": "${ip:proxy-1}", "TARGET_CONTAINER": name+"-postgres", "PROBE_CONTAINER": name+"-ha-probe"},
            "mounts": [{"source": "/var/run/docker.sock", "target": "/var/run/docker.sock"}],
            "healthcheck": {"cmd": ["/ha-probe", "health"], "interval": "1s", "retries": 10},
        })
    execution["containers"].append({"name": "proxy-1-haproxy", "set": {"image": "mirror.gcr.io/library/haproxy@sha256:ff001ac3d6df60921a5e53a9ba1ec9d4fa0edd701b4f851cecbd46b42fa92f66"}})
    for name in etcds:
        execution["containers"].append({"name": name+"-etcd", "set": {"image": "quay.io/coreos/etcd@sha256:d367cba7801b29d2f7481bb56802894658ec8a647834509118c81f77f721381b"}})
    for role in ["db", "db-replica"]:
        for to, port in [("db", 5432), ("db-replica", 5432), ("proxy", 5000)]:
            execution["flows"].append({"from_role": role, "to_role": to, "protocol": "tcp", "port": port, "label": "ha-probe"})
    segments = []
    for name, query, duration in [("start-ha-probe", "INSERT INTO live_ha_markers VALUES (0,'before-fault',inet_server_addr()::text);", None),
                                   ("during-failover", "CALL live_ha_tick();", "4m"), ("verify-ha-probe", "CALL live_ha_verify();", None)]:
        segments.append({"name": name, "workload": {"script": "execute_sql", "sql_body": query}, "timeout": "8m", "log_level": "info",
                         "thresholds": {"error_rate": 1 if duration else 0},
                         "run": {"executor": "constant-vus", "vus": 1, "duration": duration, "query_timeout": "5s"} if duration else
                                {"executor": "shared-iterations", "vus": 1, "iterations": 1, "query_timeout": "5s"}})
    workload = {"schema": {"id": "workload.stroppy", "version": "1"}, "protocol": "pg", "stroppy_version": "6.0.0", "segments": segments}
    test = (json.loads((dest/"test-created.json").read_text())["body"] if a.update_existing else
            call("test-created", "/tests/"+a.source_test+":clone", "POST", {"name": a.name}))
    test = call("test", "/tests/"+test["id"], "PATCH", {"database": {"inline": database}, "workload": {"inline": workload},
                                                              "sizes": sizes, "execution": execution, "keep": "0s"})
    assert test["validation"]["fits"], "test invalid; inspect evidence"
    print("prepared", test["id"])


if __name__ == "__main__":
    main()
