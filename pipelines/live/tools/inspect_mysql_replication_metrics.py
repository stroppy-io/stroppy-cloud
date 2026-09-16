#!/usr/bin/env python3
"""Check persisted semi-sync counters alongside live SQL replication evidence."""
import argparse
import datetime as dt
import json
from pathlib import Path


def inspect(metrics, result, sql):
    assert sql["status"] == "passed"
    assert sql["run_id"] == metrics["run_id"]
    rows = metrics["workload_observation"]["replication"]
    source = next(v for v in rows.values() if "mysql_global_status_rpl_semi_sync_source_status" in v)
    replica = next(v for v in rows.values() if "mysql_global_status_rpl_semi_sync_replica_status" in v)
    source_prefix = "mysql_global_status_rpl_semi_sync_source_"
    for name in ("status", "clients"):
        assert source[source_prefix + name]["min"] == 1
    for name in ("no_times", "no_tx", "timefunc_failures"):
        assert source[source_prefix + name]["max"] == 0
    assert source[source_prefix + "yes_tx"]["last"] > 0
    assert replica["mysql_global_status_rpl_semi_sync_replica_status"]["min"] == 1
    for alternatives in [("mysql_slave_status_replica_io_running", "mysql_slave_status_slave_io_running"), ("mysql_slave_status_replica_sql_running", "mysql_slave_status_slave_sql_running")]:
        metric = next(replica[n] for n in alternatives if n in replica)
        assert metric["min"] == 1
    for name in ("last_errno", "last_io_errno", "last_sql_errno"):
        assert replica["mysql_slave_status_" + name]["max"] == 0
    lag = next(replica[n] for n in ("mysql_slave_status_seconds_behind_source", "mysql_slave_status_seconds_behind_master") if n in replica)
    assert lag["last"] == 0
    last_segment = dt.datetime.fromisoformat(result["segments"][-1]["finished_at"].replace("Z", "+00:00")).timestamp()
    for sample in [source[source_prefix + "status"], replica["mysql_global_status_rpl_semi_sync_replica_status"]]:
        assert sample["samples"] >= 5
        assert sample["last_at"] >= last_segment - 15
    return {
        "status": "passed",
        "run_id": metrics["run_id"],
        "source": "persisted mysqld_exporter time series through Graphene Metrics API",
        "query_range": metrics["workload_observation"]["query_range"],
        "source_counters": source,
        "replica_counters": replica,
        "last_segment_finished_at": result["segments"][-1]["finished_at"],
        "checks": ["source and replica semi-sync active in all collected samples", "one acknowledging replica", "successful acknowledgements recorded", "zero async transactions and zero semi-sync fallback events", "sample coverage reaches the last segment within one 15-second query step", "replica IO and SQL threads running, no replication errors, final apply lag zero"],
        "limitations": ["Sampled exporter observations, not continuous SQL monitoring", "Replica read_only, receiver/applier health and row delivery are checked separately by live SQL during the simple segment"],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("prefix", type=Path, help="Evidence prefix, e.g. pipelines/live/mysql84-semi-sync")
    args = parser.parse_args()
    def read(suffix):
        return json.loads(Path(str(args.prefix) + suffix + ".json").read_text())
    result = inspect(read(".metrics"), read(".result"), read(".replication"))
    Path(str(args.prefix) + ".replication-metrics.json").write_text(json.dumps(result, indent=2) + "\n")
    print(result["run_id"], "persisted semi-sync checks passed")


if __name__ == "__main__":
    main()
