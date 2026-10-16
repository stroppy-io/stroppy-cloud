#!/usr/bin/env bash
# Build version: git describe, and for a dirty tree a hash of its changes.
# The version is the pipeline revision the server pushes into tenant
# namespaces; a bare "-dirty" for every local edit would leave them on the
# first edit's pipelines.
set -euo pipefail
v=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
if [[ $v == *-dirty ]]; then
	h=$({ git diff HEAD; git ls-files --others --exclude-standard -z | xargs -0 -r cat; } 2>/dev/null | sha256sum | cut -c1-8)
	v="$v-$h"
fi
echo "$v"
