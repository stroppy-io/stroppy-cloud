#!/usr/bin/env python3
"""Validate native cluster and ProxySQL metrics during the workload window."""
import collections
import datetime
import json
from pathlib import Path
import sys


def inspect(metrics):
    observation = metrics.get("cluster_stable_observation", metrics["workload_observation"])
    if "cluster_stable_observation" in metrics:
        original = metrics["workload_observation"]
        parse = lambda value: datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
        offset = (parse(observation["query_range"]["start"]) - parse(original["query_range"]["start"])).total_seconds()
        assert 0 < offset <= 45 and metrics.get("cluster_coverage_note"), "startup coverage must be explicit and bounded"
        assert observation["query_range"]["end"] == original["query_range"]["end"], "cannot omit later cluster health"
        assert observation["run_id"] == original["run_id"] == metrics["run_id"]
    rows = observation["cluster_replication"]
    nodes = collections.defaultdict(list)
    for row in rows:
        nodes[row["metric"]["graphene.entity"]].append(row)
    assert len(nodes) == 3, "cluster metrics must cover all three database exporters"
    checks = {}
    galera_uuids = set()
    for node, series in nodes.items():
        by_name = collections.defaultdict(list)
        for row in series:
            by_name[row["metric"]["__name__"]].append(row)
        members = by_name.get("mysql_perf_schema_replication_group_member_info")
        if members:
            assert len({r["metric"]["member_id"] for r in members}) == 3
            assert all(r["metric"]["member_state"] == "ONLINE" for r in members), node
            assert len({r["metric"]["member_id"] for r in members if r["metric"]["member_role"] == "PRIMARY"}) == 1
            required = ("mysql_perf_schema_transactions_checked_total", "mysql_perf_schema_transactions_remote_applied_total", "mysql_perf_schema_transactions_in_queue")
            assert all(by_name.get(name) for name in required), list(by_name)
            assert max(r["max"] for r in by_name[required[0]]) > 0
            checks[node] = {"topology": "group-replication", "members_online": 3, "primary_count": 1, "transaction_statistics": "present"}
        else:
            for name, value in {"wsrep_cluster_size": 3, "wsrep_local_state": 4, "wsrep_ready": 1, "wsrep_connected": 1, "wsrep_cluster_status": 1}.items():
                found = by_name.get("mysql_global_status_" + name)
                assert found and all(r["min"] == r["max"] == value for r in found), (node, name, found)
            assert by_name.get("mysql_global_status_wsrep_received") and by_name.get("mysql_global_status_wsrep_replicated")
            identity = by_name.get("mysql_galera_status_info")
            assert identity, (node, "missing Galera identity")
            for row in identity:
                labels = row["metric"]
                assert labels["wsrep_local_state_uuid"] == labels["wsrep_cluster_state_uuid"]
                galera_uuids.add(labels["wsrep_cluster_state_uuid"])
            assert by_name.get("mysql_galera_evs_repl_latency_avg_seconds"), (node, "missing Galera replication latency")
            checks[node] = {"topology": "galera", "cluster_size": 3, "synced": True, "connected": True, "ready": True, "primary_component": True, "state_uuid": identity[0]["metric"]["wsrep_cluster_state_uuid"], "replication_latency": "present"}
    if galera_uuids:
        assert len(galera_uuids) == 1, galera_uuids
    proxy = observation["proxy_metrics"]
    assert proxy and len({r["metric"]["graphene.entity"] for r in proxy}) == 1
    queries = [r for r in proxy if r["metric"]["__name__"] == "proxysql_connpool_conns_queries_total"]
    assert queries and sum(r["max"] for r in queries) > 0
    for row in proxy:
        if row["metric"]["__name__"].startswith("proxysql_access_denied_"):
            assert row["max"] == 0, row
    return {"status": "passed", "run_id": metrics["run_id"], "checked_at": datetime.datetime.now(datetime.timezone.utc).isoformat(), "coverage_note": metrics.get("cluster_coverage_note", "entire workload window at available collection timestamps"), "source": observation["source"], "query_range": observation["query_range"], "nodes": checks, "proxy": {"series": len(proxy), "backend_queries_observed": True, "access_denied": 0}}


if __name__ == "__main__":
    prefix = Path(sys.argv[1])
    result = inspect(json.loads(prefix.with_name(prefix.name + ".metrics.json").read_text()))
    prefix.with_name(prefix.name + ".replication-metrics.json").write_text(json.dumps(result, indent=2) + "\n")
    print(prefix, "cluster and proxy metrics passed")
