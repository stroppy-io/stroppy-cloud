#!/usr/bin/env python3
"""Build a traceable dev image from a Stroppy working tree; never read registry secrets."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tarfile


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--source", required=True)
    p.add_argument("--evidence-dir", required=True)
    p.add_argument("--context-dir", required=True)
    p.add_argument("--repository", default="ghcr.io/stroppy-io/stroppy")
    p.add_argument("--version-prefix", default="dev-tpcds")
    a = p.parse_args()
    source, dest, ctx = map(lambda x: Path(x).resolve(), [a.source, a.evidence_dir, a.context_dir])
    dest.mkdir(parents=True, exist_ok=True)
    ctx.mkdir(parents=True, exist_ok=True)
    if (dest/"image.json").exists():
        raise RuntimeError("build already recorded; choose a new evidence directory")

    def git(*args):
        return subprocess.check_output(["git", *args], cwd=source, text=True).splitlines()

    files = sorted(set(git("diff", "HEAD", "--name-only") + git("ls-files", "--others", "--exclude-standard")))
    manifest = {"base_commit": git("rev-parse", "HEAD")[0],
                "files": {f: sha(source/f) if (source/f).is_file() else None for f in files}}
    manifest["source_hash"] = hashlib.sha256(json.dumps(manifest, sort_keys=True).encode()).hexdigest()
    if not a.version_prefix.replace("-", "").isalnum():
        p.error("version prefix must contain only letters, digits and hyphens")
    manifest["version"] = a.version_prefix + "-" + manifest["source_hash"][:12]
    with tarfile.open(dest/"source-changes.tar.gz", "w:gz") as archive:
        for f in files:
            if (source/f).is_file():
                archive.add(source/f, arcname=f)
    manifest["source_archive_sha256"] = sha(dest/"source-changes.tar.gz")
    with (dest/"build.log").open("w") as log:
        subprocess.run(["make", "build", "VERSION="+manifest["version"]], cwd=source, stdout=log, stderr=subprocess.STDOUT, check=True)
    assert manifest["files"] == {f: sha(source/f) if (source/f).is_file() else None for f in files}, "source changed while building"
    shutil.copy2(source/"build/stroppy", ctx/"stroppy")
    manifest["binary_sha256"] = sha(ctx/"stroppy")
    manifest["tag"] = a.repository + ":" + manifest["version"]
    dockerfile = "FROM alpine:3.22\nRUN apk add --no-cache ca-certificates\nWORKDIR /workspace\nCOPY stroppy /usr/local/bin/stroppy\nENTRYPOINT [\"/usr/local/bin/stroppy\"]\n"
    (ctx/"Dockerfile").write_text(dockerfile)
    (dest/"Dockerfile").write_text(dockerfile)
    with (dest/"image-build.log").open("w") as log:
        subprocess.run(["docker", "build", "-t", manifest["tag"], str(ctx)], stdout=log, stderr=subprocess.STDOUT, check=True)
    actual = subprocess.check_output(["docker", "run", "--rm", "--entrypoint", "sha256sum", manifest["tag"], "/usr/local/bin/stroppy"], text=True).split()[0]
    assert actual == manifest["binary_sha256"]
    manifest["image_id"] = subprocess.check_output(["docker", "image", "inspect", "--format={{.Id}}", manifest["tag"]], text=True).strip()
    (dest/"image.json").write_text(json.dumps(manifest, indent=2)+"\n")
    print(json.dumps({k: manifest[k] for k in ["version", "tag", "binary_sha256", "source_hash"]}))


if __name__ == "__main__":
    main()
