#!/usr/bin/env python3
"""Check schemas against local Stroppy. No server, Docker, or cloud resources.

--refresh replaces the independent fixtures; review their diff before accepting
an upstream change. A fixture refresh is not a successful compatibility test.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]
FIXTURES = ROOT / "pipelines/stroppycfg/testdata"


def capture(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, timeout=60)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stroppy-root", type=Path, default=ROOT.parent / "stroppy")
    parser.add_argument("--binary", type=Path)
    parser.add_argument("--refresh", action="store_true")
    args = parser.parse_args()
    source = args.stroppy_root.resolve()
    binary = (args.binary or source / "build/stroppy").resolve()
    probe = capture(str(binary), "probe", "-o", "json")
    native_schema = (source / "docs/jsonschema/run.schema.json").read_bytes()
    fixtures = {"stroppy-probe.json": probe, "stroppy-run.schema.json": native_schema}
    if args.refresh:
        for name, data in fixtures.items():
            (FIXTURES / name).write_bytes(data)
        build_info = capture("go", "version", "-m", str(binary)).decode()
        with binary.open("rb") as stream:
            binary_hash = hashlib.file_digest(stream, "sha256").hexdigest()
        metadata = {
            "repository": "https://github.com/stroppy-io/stroppy",
            "source_commit": capture("git", "rev-parse", "HEAD", cwd=source).decode().strip(),
            "source_dirty": bool(capture("git", "status", "--porcelain", cwd=source).strip()),
            "binary_sha256": binary_hash,
            "binary_vcs": [line.strip() for line in build_info.splitlines() if "vcs." in line],
            "fixture_sha256": {name: hashlib.sha256(data).hexdigest() for name, data in fixtures.items()},
            "scope": "Binary probe and source-generated JSON Schema; executable checks use noop only. Source commit does not assert binary provenance; binary VCS metadata is recorded separately.",
        }
        (FIXTURES / "stroppy-contract-source.json").write_text(json.dumps(metadata, indent=2) + "\n")
    else:
        for name, data in fixtures.items():
            if json.loads(data) != json.loads((FIXTURES / name).read_bytes()):
                raise SystemExit(f"{name} differs from local Stroppy; inspect the change before --refresh")
    env = dict(os.environ, STROPPY_CONTRACT_BINARY=str(binary))
    subprocess.run(
        ["go", "test", "./spec", "./stroppycfg", "./internal/activities", "./internal/run", "./internal/suite", "-count=1"],
        cwd=ROOT / "pipelines", env=env, check=True, timeout=600,
    )


if __name__ == "__main__":
    main()
