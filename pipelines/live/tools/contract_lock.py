#!/usr/bin/env python3
"""Check the reviewed server/UI/pipeline contract snapshot without cloud access."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[3]
LOCK = ROOT / "pipelines/live/tests/platform/contracts/contract.lock.json"


def snapshot():
    version_file = ROOT / "pipelines/spec/contract_version.go"
    version = re.search(r'ContractVersion = "([^"]+)"', version_file.read_text())[1]
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("ContractVersion must be semver")
    api = (ROOT / "openapi/openapi.yaml").read_text()
    if f"x-pipeline-contract-version: {version}" not in api:
        raise ValueError("OpenAPI and pipeline contract versions disagree; regenerate OpenAPI")
    patterns = [
        "pipelines/schemas/testdata/*.json", "web/src/schemas/*.json", "web/src/schemas/*.ts",
        "pipelines/spec/spec.go", "pipelines/spec/segment.go", "pipelines/spec/result.go",
        "pipelines/spec/contract_version.go", "pipelines/go.mod",
        "pipelines/stroppycfg/testdata/*.json", "openapi/openapi.yaml",
        "web/src/api/schema.d.ts", "web/src/api/json.ts",
        "pipelines/live/tests/platform/contracts/result-complete.json",
        "pipelines/live/tests/platform/contracts/test-patch.json",
    ]
    files = {}
    for pattern in patterns:
        paths = sorted(ROOT.glob(pattern))
        if not paths:
            raise ValueError(f"missing contract artifact: {pattern}")
        for path in paths:
            raw = path.read_bytes()
            if path.suffix == ".json":
                raw = json.dumps(json.loads(raw), sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()
            files[path.relative_to(ROOT).as_posix()] = hashlib.sha256(raw).hexdigest()
    return {"version": version, "hash": "sha256 (JSON canonicalized)", "files": dict(sorted(files.items()))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true", help="record an intentionally reviewed contract version")
    args = parser.parse_args()
    current = snapshot()
    old = json.loads(LOCK.read_text()) if LOCK.exists() else None
    if args.write:
        if old and old != current and old["version"] == current["version"]:
            raise ValueError("contract changed under the same version; review compatibility and bump ContractVersion + OpenAPI extension")
        LOCK.write_text(json.dumps(current, indent=2) + "\n")
    elif current != old:
        previous = (old or {}).get("files", {})
        changed = sorted(k for k in set(previous) | set(current["files"]) if previous.get(k) != current["files"].get(k))
        for name in changed:
            print(f"contract drift: {name}", file=sys.stderr)
        raise ValueError("contract snapshot differs; follow HANDOFF.md before updating the lock")
    print(f"contract {current['version']}: {len(current['files'])} artifacts match")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError) as error:
        sys.exit(str(error))
