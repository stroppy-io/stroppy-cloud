#!/usr/bin/env python3
"""Inspect persisted pipeline traces for an explicit time range (no retention changes)."""
import argparse
import datetime
import json
from pathlib import Path
import urllib.parse
import urllib.request


def inspect(run_id, namespace, endpoint, start, end):
    def get(path):
        with urllib.request.urlopen(endpoint.rstrip("/") + path, timeout=30) as response:
            return json.load(response)

    def micros(value):
        parsed = datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
        if parsed.tzinfo is None:
            raise ValueError("time range requires timezone offsets")
        return int(parsed.timestamp() * 1_000_000)

    traces = {}
    for service in get("/api/services")["data"]:
        query = urllib.parse.urlencode({
            "service": service, "tags": json.dumps({"temporalWorkflowID": "run/" + run_id}),
            "start": micros(start), "end": micros(end), "limit": 1000,
        })
        for trace in get("/api/traces?" + query).get("data", []):
            namespaces = {tag.get("value") for process in trace.get("processes", {}).values()
                          for tag in process.get("tags", []) if tag["key"] == "graphene.namespace"}
            if namespace in namespaces:
                traces[trace["traceID"]] = trace
    operations = set()
    spans = errors = 0
    for trace in traces.values():
        for span in trace.get("spans", []):
            tags = {tag["key"]: tag.get("value") for tag in span.get("tags", [])}
            process = trace.get("processes", {}).get(span.get("processID"), {})
            resource = {tag["key"]: tag.get("value") for tag in process.get("tags", [])}
            if tags.get("temporalWorkflowID") == "run/" + run_id or resource.get("graphene.run") == run_id:
                spans += 1
                errors += tags.get("error") is True
                operations.add(span["operationName"])
    return {"run_id": run_id, "source": "persistent VictoriaTraces Jaeger API", "start": start, "end": end,
            "trace_ids": sorted(traces), "trace_count": len(traces), "matching_spans": spans,
            "error_spans": errors, "operations": sorted(operations),
            "scope": "pipeline activities; does not prove Stroppy workload OTLP export"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("run_id")
    parser.add_argument("--namespace", default="t-stroppy-live")
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--start", required=True)
    parser.add_argument("--end", required=True)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    result = inspect(args.run_id, args.namespace, args.endpoint, args.start, args.end)
    body = json.dumps(result, indent=2) + "\n"
    if args.output:
        args.output.write_text(body)
    print(body, end="")


if __name__ == "__main__":
    main()
