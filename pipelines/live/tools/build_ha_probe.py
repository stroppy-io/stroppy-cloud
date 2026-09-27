#!/usr/bin/env python3
"""Build the isolated HA test controller using the local Stroppy module's pinned driver."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--stroppy", required=True)
    p.add_argument("--evidence-dir", required=True)
    p.add_argument("--push", action="store_true")
    a = p.parse_args()
    source = Path(__file__).with_name("ha_failover_probe.go")
    dest = Path(a.evidence_dir)
    dest.mkdir(parents=True, exist_ok=True)
    sha = hashlib.sha256(source.read_bytes()).hexdigest()
    tag = "ghcr.io/stroppy-io/stroppy:dev-ha-probe-" + sha[:12]
    env = dict(os.environ, CGO_ENABLED="0")
    with tempfile.TemporaryDirectory(prefix="stroppy-ha-probe-") as tmp:
        tmp = Path(tmp)
        subprocess.run(["go", "build", "-o", str(tmp/"ha-probe"), str(source)], cwd=a.stroppy, env=env, check=True)
        dockerfile = "FROM alpine:3.22\nCOPY ha-probe /ha-probe\nENTRYPOINT [\"/ha-probe\"]\n"
        (tmp/"Dockerfile").write_text(dockerfile)
        with (dest/"build.log").open("w") as log:
            subprocess.run(["docker", "build", "-t", tag, str(tmp)], stdout=log, stderr=subprocess.STDOUT, check=True)
        (dest/"Dockerfile").write_text(dockerfile)
        with tarfile.open(dest/"source-changes.tar.gz", "w:gz") as archive:
            archive.add(source, arcname=source.name)
            for name in ("go.mod", "go.sum"):
                archive.add(Path(a.stroppy)/name, arcname=name)
        manifest = {"tag": tag, "source_sha256": sha,
                    "binary_sha256": hashlib.sha256((tmp/"ha-probe").read_bytes()).hexdigest(),
                    "driver_go_mod_sha256": hashlib.sha256((Path(a.stroppy)/"go.mod").read_bytes()).hexdigest()}
    if a.push:
        with (dest/"push.log").open("w") as log:
            subprocess.run(["docker", "push", tag], stdout=log, stderr=subprocess.STDOUT, check=True)
        inspect = json.loads(subprocess.check_output(["skopeo", "inspect", "--no-creds", "docker://"+tag]))
        manifest["pinned_image"] = tag.split(":", 1)[0] + "@" + inspect["Digest"]
    (dest/"image.json").write_text(json.dumps(manifest, indent=2)+"\n")
    print(json.dumps(manifest))


if __name__ == "__main__":
    main()
