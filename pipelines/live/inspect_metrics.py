#!/usr/bin/env python3
"""Verify persisted run metrics through Graphene, without workload credentials."""
import argparse
import base64
import collections
import datetime
import math
import json
from pathlib import Path
import subprocess
import urllib.parse
import urllib.request


def inspect(run_id, namespace, *, endpoint=None, start=None, end=None, entities=None, raw_samples=False):
    source = "Graphene metrics API backed by the persistent metrics store"
    nonfinite_samples = collections.Counter()
    if raw_samples and not endpoint:
        raise ValueError("raw samples require an explicit VictoriaMetrics endpoint")
    if endpoint:
        if not start or not end:
            raise ValueError("explicit backend queries require both start and end")
        selector = "{" + json.dumps("graphene.namespace") + "=" + json.dumps(namespace) + "," + json.dumps("graphene.run") + "=" + json.dumps(run_id) + "}"
        if raw_samples:
            merged = {}
            lower = datetime.datetime.fromisoformat(start.replace("Z", "+00:00")).timestamp() * 1000
            upper = datetime.datetime.fromisoformat(end.replace("Z", "+00:00")).timestamp() * 1000
            for entity in entities or [None]:
                match = selector[:-1] + ("," + json.dumps("graphene.entity") + "=" + json.dumps(entity) if entity else "") + "}"
                query = urllib.parse.urlencode({"match[]": match, "start": start, "end": end})
                with urllib.request.urlopen(endpoint.rstrip("/") + "/api/v1/export?" + query, timeout=30) as response:
                    for line in response:
                        if not line.strip():
                            continue
                        row = json.loads(line)
                        key = tuple(sorted(row["metric"].items()))
                        target = merged.setdefault(key, {"metric": row["metric"], "values": []})
                        assert len(row["timestamps"]) == len(row["values"])
                        for stamp, value in zip(row["timestamps"], row["values"]):
                            if not lower <= stamp <= upper:
                                continue
                            # VictoriaMetrics represents non-finite raw samples
                            # as JSON null. Retain their counts for diagnostics.
                            if value is None or not math.isfinite(value):
                                nonfinite_samples[(row["metric"].get("graphene.entity", ""), row["metric"].get("__name__", ""))] += 1
                                continue
                            target["values"].append((stamp / 1000, value))
            for row in merged.values():
                row["values"] = sorted(set(row["values"]))
            snapshot = {"status": "success", "data": {"result": list(merged.values())}}
            source = "persistent VictoriaMetrics raw export with original sample timestamps; no range-query lookback or resampling"
        else:
            query = urllib.parse.urlencode({"query": selector, "start": start, "end": end, "step": "15s"})
            with urllib.request.urlopen(endpoint.rstrip("/") + "/api/v1/query_range?" + query, timeout=30) as response:
                snapshot = json.load(response)
            source = "persistent VictoriaMetrics API with explicit time range"
    else:
        series = []
        for target in entities or ["run/" + run_id]:
            kind, entity_id = target.split("/", 1)
            command = ["graphenectl", "-n", namespace, "metrics", kind, entity_id, "--jq", "."]
            if start:
                command += ["--start", start]
            if end:
                command += ["--end", end]
            raw = subprocess.check_output(command, text=True, timeout=30)
            part = json.loads(base64.b64decode(json.loads(raw)["snapshot"]))
            if part.get("status") != "success":
                raise RuntimeError("metrics backend did not return success")
            series.extend(part["data"]["result"])
        snapshot = {"status": "success", "data": {"result": series}}
        if entities:
            source += "; separate queries for each exporter resource"
    if snapshot.get("status") != "success":
        raise RuntimeError("metrics backend did not return success")
    series = snapshot["data"]["result"]
    names = collections.defaultdict(set)
    disks = []
    filesystems = []
    exporter_runs = set()
    exporter_agents = set()
    failures = set()
    postgres = set()
    mysql = set()
    mysql_collectors = []
    component_health = {}
    replication = {}
    cluster_replication = []
    proxy_metrics = []
    database_versions = {}
    transfer_attempts = set()
    for row in series:
        metric = row["metric"]
        name = metric.get("__name__", "")
        entity = metric.get("graphene.entity", "")
        samples = row.get("values", [row["value"]] if "value" in row else [])
        if not samples:
            continue
        values = [float(value) for _, value in samples]
        # CockroachDB and YDB do not use one common metric-name prefix.
        # Require the compiled database entity plus exporter-specific labels.
        cockroach = entity.endswith("-cockroach") and bool(metric.get("node_id"))
        ydb = entity.endswith("-ydb") and bool(metric.get("counters"))
        if cockroach or ydb or name.startswith(("node_", "pg_", "pgbouncer_", "haproxy_", "patroni_", "etcd_", "mysql_", "proxysql_", "pico_")):
            names[entity].add(name)
            exporter_runs.add(metric.get("graphene.run", ""))
            exporter_agents.add(metric.get("graphene.agent", ""))
        if name in ("pg_up", "pgbouncer_up", "pg_replication_is_replica", "pg_replication_lag_seconds", "patroni_primary", "patroni_postgres_running", "etcd_server_has_leader", "etcd_server_is_leader", "mysql_up", "mysql_slave_status_slave_io_running", "mysql_slave_status_slave_sql_running", "mysql_slave_status_seconds_behind_master"):
            target = replication if name.startswith(("pg_replication_", "mysql_slave_status_")) else component_health
            target.setdefault(entity, {})[name] = {"min": min(values), "max": max(values)}
        if name.startswith(("mysql_global_status_rpl_semi_sync_", "mysql_slave_status_")):
            replication.setdefault(entity, {})[name] = {"min": min(values), "max": max(values), "last": values[-1], "samples": len(samples), "first_at": samples[0][0], "last_at": samples[-1][0]}
        if name.startswith(("mysql_perf_schema_replication_group_", "mysql_perf_schema_transactions_", "mysql_perf_schema_conflicts_", "mysql_global_status_wsrep_", "mysql_galera_", "proxysql_")):
            target = proxy_metrics if name.startswith("proxysql_") else cluster_replication
            target.append({"metric": metric, "min": min(values), "max": max(values), "last": values[-1], "samples": len(samples), "first_at": samples[0][0], "last_at": samples[-1][0]})
        if name == "pico_instance_state" and metric.get("state") == "Online":
            component_health.setdefault(entity, {})["picodata_online"] = {"min": min(values), "max": max(values), "samples": len(values)}
        if name == "pico_raft_leader_id":
            component_health.setdefault(entity, {})[name] = {"min": min(values), "max": max(values), "samples": len(values)}
        if cockroach and name in ("liveness_livenodes", "node_id", "sys_uptime", "sql_conns"):
            component_health.setdefault(entity, {})[name] = {"min": min(values), "max": max(values), "samples": len(values)}
        if cockroach and name == "build_timestamp":
            database_versions[entity] = {"version": metric.get("tag", ""), "node_id": metric["node_id"]}
        if name == "pg_static":
            database_versions[entity] = {k: metric.get(k, "") for k in ("short_version", "version")}
        if name == "mysql_version_info":
            database_versions[entity] = {k: metric.get(k, "") for k in ("version", "version_comment")}
        if name == "node_disk_info":
            disks.append({"entity": entity, **{k: metric.get(k, "") for k in ("device", "serial", "model", "path")}})
        if name == "node_filesystem_size_bytes" and metric.get("mountpoint") == "/data":
            filesystems.append({"entity": entity, "device": metric.get("device"), "fstype": metric.get("fstype"), "size_bytes": max(values)})
        if name == "node_scrape_collector_success" and min(values) < 1:
            failures.add((entity, metric.get("collector", "")))
        if name == "pg_exporter_last_scrape_error":
            postgres.add((entity, max(values)))
        if name == "mysql_exporter_collector_success":
            mysql_collectors.append({"entity": entity, "collector": metric.get("collector", ""), "min": min(values), "max": max(values), "samples": len(values), "first_at": samples[0][0], "last_at": samples[-1][0]})
        if name == "mysql_exporter_last_scrape_error":
            mysql.add((entity, max(values)))
        if metric.get("graphene.activity") == "server.resource.transfer":
            transfer_attempts.add(metric.get("graphene.attempt", ""))
    return {
        "run_id": run_id,
        "source": source,
        "query_range": {"start": start, "end": end} if start or end else "Graphene default: last hour",
        "series_count": len(series),
        "nonfinite_raw_samples": [{"entity": entity, "metric": name, "samples": count}
                                  for (entity, name), count in sorted(nonfinite_samples.items())],
        "database_distribution_scope": "Scalar series only: Graphene docker 0.2.2 and 0.2.3 drop histogram/summary families and counter types; see database-metric-coverage.json",
        "queried_entities": entities or ["run/" + run_id],
        "exporter_runs_observed": sorted(exporter_runs),
        "exporter_agents_observed": sorted(exporter_agents),
        # Retry attempts produce separate telemetry series for the same mount.
        # Collapse only identical inventory records; conflicting sizes, devices
        # or filesystem types remain separate and fail the expected-disk check.
        "data_filesystems": list({tuple(sorted(row.items())): row for row in filesystems}.values()),
        "data_filesystem_series_count": len(filesystems),
        "exporters": {k: {"metric_names": len(v)} for k, v in sorted(names.items())},
        "component_health": component_health,
        "replication": replication,
        "cluster_replication": cluster_replication,
        "proxy_metrics": proxy_metrics,
        "database_versions": database_versions,
        "disk_info": sorted(disks, key=lambda x: (x["entity"], x["device"])),
        "node_collectors_reporting_zero": sorted(failures),
        "node_collector_zero_note": "Zero can also mean hardware or subsystem absent; inspect exporter logs before treating it as a fault.",
        "postgres_exporter_scrape_error_max": sorted(postgres),
        "mysql_exporter_scrape_error_max": sorted(mysql),
        "mysql_exporter_collectors": sorted(mysql_collectors, key=lambda row: (row["entity"], row["collector"])),
        "transfer_activity_attempts_observed": sorted(transfer_attempts),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("run_id")
    parser.add_argument("--namespace", default="t-stroppy-live")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--endpoint")
    parser.add_argument("--raw-samples", action="store_true", help="Use original VictoriaMetrics samples in the exact time window")
    parser.add_argument("--entity", action="append", help="Read each exporter resource separately (kind/id)")
    parser.add_argument("--start")
    parser.add_argument("--end")
    args = parser.parse_args()
    result = inspect(args.run_id, args.namespace, endpoint=args.endpoint, start=args.start, end=args.end, entities=args.entity, raw_samples=args.raw_samples)
    body = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.write_text(body)
    print(body, end="")


if __name__ == "__main__":
    main()
