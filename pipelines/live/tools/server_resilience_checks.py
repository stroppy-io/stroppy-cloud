#!/usr/bin/env python3
"""Run offline recovery gates and record the unexecuted integration/live boundary."""
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys

from server_acceptance import ROOT, source_digest


def main():
    dest = ROOT / "pipelines/live/tests/platform/server/resilience"
    private = ROOT / ".local/server-resilience"
    private.mkdir(parents=True, exist_ok=True)
    before = source_digest()
    checks = [
        ("recovery-race", ROOT, ["go", "test", "-json", "-race", "-count=1", "./internal/domain/run", "./internal/domain/suite", "./internal/infrastructure/graphene", "-run", "^(TestRecovery|TestRunRecovery)"]),
        ("yc-auth-local", ROOT / "pipelines", ["go", "test", "-json", "-count=1", "./internal/cloud", "./internal/activities", "-run", "Test.*(Yandex|Token|Runtime|YDB)"]),
        ("integration-compile", ROOT, ["go", "test", "-c", "-tags=integration", "-o", str(private / "integration.test"), "./cmd/stroppy-cloud/application"]),
        ("lint", ROOT, ["golangci-lint", "run", "./..."]),
    ]
    results = {}
    for name, cwd, command in checks:
        cases = {}
        log = private / (name + ".log")
        with log.open("w") as out:
            os.chmod(log, 0o600)
            proc = subprocess.Popen(command, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
            for line in proc.stdout:
                out.write(line)
                try:
                    event = json.loads(line)
                except ValueError:
                    continue
                if event.get("Test") and event.get("Action") in {"pass", "fail", "skip"}:
                    cases[event["Package"] + "/" + event["Test"]] = event["Action"]
            code = proc.wait()
        results[name] = {"command": command, "exit_code": code, "tests": cases}
        print(name, "passed" if code == 0 else "failed", len(cases), flush=True)
    report = {
        "observed_at": datetime.now(timezone.utc).isoformat(),
        "source_sha256_before": before, "source_sha256_after": source_digest(),
        "checks": results,
        "integration_execution": "not_run_by_this_tool: see separate acceptance/resilience-integration.json and resilience-race.json",
        "live_execution": "not_run_by_this_tool: see separate resilience/live-suite-20260927 evidence",
        "yc_token_renewal_live": "not_run: local credential-file checks do not prove renewal against IAM/Crossplane",
    }
    dest.mkdir(parents=True, exist_ok=True)
    (dest / "local-checks.json").write_text(json.dumps(report, indent=2) + "\n")
    return int(any(v["exit_code"] != 0 for v in results.values()) or before != report["source_sha256_after"])


if __name__ == "__main__":
    sys.exit(main())
