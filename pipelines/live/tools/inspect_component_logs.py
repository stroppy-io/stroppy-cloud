#!/usr/bin/env python3
"""Count persisted component logs for an exact run; retain no raw log bodies."""
import argparse
import collections
import datetime
import hashlib
import re
import json
from pathlib import Path
import urllib.parse
import urllib.request


def inspect(run_id, namespace, endpoint, *, start=None, end=None):
    query = json.dumps("graphene.namespace") + ":=" + json.dumps(namespace)
    query += " AND " + json.dumps("graphene.run") + ":=" + json.dumps(run_id)
    # VictoriaLogs streams every matching row when limit is omitted.
    # Consume incrementally: large multi-node runs must not truncate at 20k.
    body = urllib.parse.urlencode({"query": query}).encode()
    request = urllib.request.Request(endpoint.rstrip("/") + "/select/logsql/query", data=body)
    candidates = []
    observations = []
    lower = upper = None
    if start and end:
        lower = datetime.datetime.fromisoformat(start.replace("Z", "+00:00"))
        upper = datetime.datetime.fromisoformat(end.replace("Z", "+00:00"))
    error = re.compile(r'\[(?:ERROR|FATAL)\]|level=(?:ERROR|FATAL)|"level"\s*:\s*"(?:error|fatal)"|\b(?:ERROR|FATAL|PANIC):|\s[EF]>\s|^[EF][0-9]{6}\s')
    entities = collections.Counter()
    runs, namespaces = set(), set()
    records = 0
    with urllib.request.urlopen(request, timeout=30) as response:
        for line in response:
            if not line.strip():
                continue
            row = json.loads(line)
            if row.get("graphene.run") != run_id or row.get("graphene.namespace") != namespace:
                raise RuntimeError("log backend returned a row outside the requested identity")
            records += 1
            entities[row.get("graphene.entity", "")] += 1
            runs.add(row["graphene.run"])
            namespaces.add(row["graphene.namespace"])
            if lower is None or upper is None:
                continue
            if not row.get("graphene.entity", "").startswith("docker/"):
                continue
            timestamp = datetime.datetime.fromisoformat(row["_time"].replace("Z", "+00:00"))
            message = row.get("_msg", "")
            if lower <= timestamp <= upper and error.search(message):
                item = {"entity": row["graphene.entity"], "observed_at": row["_time"], "message_sha256": hashlib.sha256(message.encode()).hexdigest()}
                if row["graphene.entity"].endswith("-mariadb") and re.search(r"\[ERROR\] Got error 123 when reading table '[^']+'$", message):
                    item["classification"] = "mariadb_record_changed"
                    item["reference"] = "https://jira.mariadb.org/browse/MDEV-37085"
                    observations.append(item)
                elif row["graphene.entity"].endswith("-proxysql") and "[ERROR] Timeout on Galera health check for " in message:
                    item["classification"] = "galera_monitor_timeout"
                    observations.append(item)
                elif row["graphene.entity"].endswith(("-postgres", "-orioledb")) and re.search(r"ERROR:\s+(?:could not serialize access due to concurrent (?:update|delete)|deadlock detected)\s*$", message):
                    item["classification"] = "postgres_transaction_conflict"
                    item["reference"] = "https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html"
                    observations.append(item)
                elif row["graphene.entity"].endswith(("-postgres", "-orioledb")) and re.search(r"ERROR:\s+tpcc_rollback:item_not_found\s*$", message):
                    item["classification"] = "tpcc_expected_new_order_rollback"
                    observations.append(item)
                else:
                    candidates.append(item)
    return {"run_id": run_id, "source": "persistent VictoriaLogs API, exact run and namespace selector",
            "records": records, "limit": None, "read_mode": "stream_until_eof", "runs_observed": sorted(runs),
            "namespaces_observed": sorted(namespaces),
            "component_logs": {key: count for key, count in sorted(entities.items()) if key.startswith("docker/")},
            "error_scan_window": {"start": start, "end": end} if start and end else None,
            "workload_error_candidates": candidates,
            "workload_observations": observations,
            "observation_counts": dict(collections.Counter(row["classification"] for row in observations)),
            "error_scan_note": "ERROR/FATAL markers in container logs, bounded by log observation time; unknown candidates block verification; classified observations remain visible and require independent native-error and cluster-health checks"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("run_id")
    parser.add_argument("--namespace", default="t-stroppy-live")
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    result = inspect(args.run_id, args.namespace, args.endpoint)
    args.output.write_text(json.dumps(result, indent=2) + "\n")
    print("records:", result["records"], "components:", len(result["component_logs"]))


if __name__ == "__main__":
    main()
