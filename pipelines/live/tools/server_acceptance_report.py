#!/usr/bin/env python3
"""Map server operations/capabilities to explicit local and real acceptance evidence."""
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import yaml
from server_acceptance import ROOT, DEST, source_digest


CAPABILITIES = [
    ("catalog_and_schemas", "Catalog identities, schema validation/rendering and examples", ["Catalog","IdentityHTTP","Examples"], ["public-catalog.json"]),
    ("catalog_matrix", "Every deployable database version/topology; compatible scripts on each default database", ["DatabaseMatrix"], []),
    ("runtime_inputs", "Machine, disk, network, container, file content, driver, workload, telemetry overrides survive save/edit/compile", ["CompleteRuntimeRoundtrip","RuntimeOverrides"], ["pg-run.json"]),
    ("workload_inputs", "All 9 workload variants and every discriminated parameter reach RunSpec", ["WorkloadParameterTransport"], []),
    ("lossless_contract", "Browser roundtrip preserves uint64, zero/false/empty/null and complete native result envelopes", ["ContractHandoff"], []),
    ("invalid_inputs", "Invalid disks, missing roles, file paths, conflicting SQL and impossible capacity never launch", ["InvalidExecutionNeverLaunches","ContractHandoff"], []),
    ("library_portability", "CRUD, clone, export/import, diff and immutable run snapshots", ["Library","PortableLibrary"], []),
    ("identity_and_access", "Tenant isolation, roles, token scope, invite accept/decline, IAM events and admin", ["MembersAndRoles","Tokens","Invites","IdentityHTTP","IAMWebhook","Admin"], []),
    ("provider_lifecycle", "Credentials, verification, YC quota projection, rotation, retirement and cleanup protections", ["ProviderProfiles","ProviderLifecycle","TenantProviderCleanup","QuotaVisibility"], ["server-status.json"]),
    ("concurrent_admission", "Same-key HTTP requests return one run; six competing launches respect tenant limit three", ["ConcurrentLaunch"], []),
    ("lifecycle", "Completion, failure, cancellation, partial results, full rerun, keep/release, artifacts", ["Runs","Scenarios"], ["pg-lifecycle.json","cancel-lifecycle.json","suite-rerun-verification.json","pg-artifacts.json","failure-artifacts.json","pg-cleanup.json","cancel-cleanup.json","failure-cleanup.json","suite-success-cleanup.json","rerun-cleanup.json"]),
    ("restart_recovery", "Persisted cursor resumes observation without relaunch or duplicate milestones", ["ObserverRecovery"], ["pg-lifecycle.json"]),
    ("kept_reconciliation", "Expired stand or lost release acknowledgement clears stale keep without removing artifacts", ["KeptStandRecovery"], ["pg-after-release.json","pg-artifacts.json"]),
    ("cleanup_phase", "Pipeline announces actual teardown after collection, before terminal closure", ["Scenarios"], ["rerun-teardown.json"]),
    ("final_resources", "Acceptance tenant has no active or kept runs; rerun artifacts remain downloadable", ["Runs"], ["final-resources.json","rerun-artifacts.json"]),
    ("suite_schedule_webhook", "Suite cells, retries, scheduling and webhook delivery", ["SuitesAndSchedules","Webhooks"], ["suite-rerun-verification.json"]),
    ("telemetry", "Run-scoped logs, bidirectional pagination/facets, typed/raw native and host metrics through Graphene", ["Observe","WebSocket"], ["pg-telemetry.json"]),
    ("public_share", "Published snapshots, scoped anonymous metrics, revocation, comparison and ratings", ["SharedMetrics","ShareCompareRating"], ["public-metrics.json"]),
]
UNSUPPORTED = {
    "createRunGrafanaSession": "Grafana sessions are explicitly unavailable (503); Graphene 0.2.23 integration has no session broker.",
    "createPublicShareGrafanaSession": "Same unavailable Grafana session capability for public shares (503).",
}


def main():
    local=json.loads((DEST/"local.json").read_text())
    spec=yaml.safe_load((ROOT/"openapi/openapi.yaml").read_text())
    operations={}
    for path,methods in spec["paths"].items():
        for method,op in methods.items():
            if not isinstance(op,dict) or "operationId" not in op:continue
            oid=op["operationId"]
            hits={test:counts[oid] for test,counts in local["http_observations"].items() if oid in counts}
            passed=any(int(status)//100==2 for statuses in hits.values() for status in statuses)
            operations[oid]={"method":method.upper(),"path":path,"status":"exercised" if passed else "unsupported" if oid in UNSUPPORTED else "uncovered","tests":hits}
            if oid in UNSUPPORTED:operations[oid]["reason"]=UNSUPPORTED[oid]
    capabilities=[]
    for cid,scope,tests,live in CAPABILITIES:
        tests=["TestE2E"+t for t in tests]
        ok=all(local["tests"].get(t,{}).get("status")=="pass" for t in tests)
        evidence=[]
        for file in live:
            p=DEST/file
            value=json.loads(p.read_text()) if p.exists() else None
            if value is None:
                status="missing"
            elif "verified" in value:
                status="passed" if value["verified"] is True else "failed"
            else:
                ok=200 <= value.get("http_status",0) < 300
                body=value.get("body",{})
                if file=="pg-run.json":ok=ok and body.get("status")=="completed" and bool(body.get("result",{}).get("segments"))
                if file=="pg-after-release.json":ok=ok and body.get("stand_kept") is False
                if file=="public-catalog.json":ok=ok and bool(body.get("databases")) and bool(body.get("providers"))
                if file=="server-status.json":ok=ok and body.get("components",{}).get("graphene",{}).get("status")=="ok"
                status="passed" if ok else "failed"
            evidence.append({"path":file,"status":status})
        capabilities.append({"id":cid,"scope":scope,"local_status":"passed" if ok else "failed","tests":tests,"live_evidence":evidence})
    uncovered=[k for k,v in operations.items() if v["status"]=="uncovered"]
    local_ok=(local["exit_code"]==0 and not local["source_changed_during_run"] and local["source_sha256_after"]==source_digest() and not uncovered and all(c["local_status"]=="passed" for c in capabilities))
    race_path=DEST/"races.json"
    races=json.loads(race_path.read_text()) if race_path.exists() else {}
    race_ok=(races.get("exit_code")==0 and races.get("source_changed_during_run") is False and races.get("source_sha256_after")==source_digest())
    live_ok=all(e["status"] not in {"missing","failed"} for c in capabilities for e in c["live_evidence"])
    fixtures={n:hashlib.sha256((DEST/n).read_bytes()).hexdigest() for n in ["runtime-fixture.json","workloads-fixture.json"]}
    report={"generated_at":datetime.now(timezone.utc).isoformat(),"local_verified":local_ok,"race_verified":race_ok,"live_evidence_complete":live_ok,
            "source_sha256":local["source_sha256_after"],"fixture_sha256":fixtures,"operations":operations,"capabilities":capabilities,
            "limits":["Route exercise is not a proof of every parameter combination; semantic assertions are linked separately.",
                      "Local Graphene and cloud activities are simulated; HTTP, compilation, serialization and PostgreSQL are real.",
                      "Real acceptance samples PostgreSQL and noop on YC; it does not requalify every database/topology/workload.",
                      "Stroppy uses the pinned development digest; an official release still needs separate qualification.",
                      "Grafana session endpoints are unavailable. Pipeline trace retrieval is not verified. AWS is excluded."]}
    (DEST/"capabilities.json").write_text(json.dumps(report,indent=2)+"\n")
    summary={"local_verified":local_ok,"race_verified":race_ok,"live_evidence_complete":live_ok,"operations":len(operations),"exercised":sum(v["status"]=="exercised" for v in operations.values()),"unsupported":list(UNSUPPORTED),"uncovered":uncovered,"tests":len(local["tests"])}
    print(json.dumps(summary,indent=2))
    return 0 if local_ok and race_ok and live_ok else 1


if __name__=="__main__":raise SystemExit(main())
