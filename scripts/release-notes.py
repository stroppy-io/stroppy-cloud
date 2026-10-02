"""Describe the exact container published by the release job."""
import os
from pathlib import Path

version = os.environ["RELEASE_VERSION"]
image = os.environ["RELEASE_IMAGE"]
digest = os.environ["RELEASE_DIGEST"]
if not digest.startswith("sha256:"):
    raise SystemExit("Missing container digest")
reference = f"{image}@{digest}"
Path("/tmp/image-digest.txt").write_text(reference + "\n")
Path("/tmp/release-notes.md").write_text(f"""Stroppy Cloud {version}

```sh
docker pull {image}:{version}
```

One Linux amd64 image contains the Go server with its embedded production UI
and the five matching Graphene pipeline binaries. No separate UI container or
runtime Node.js is required. The image runs as non-root and listens on port 18347.
PostgreSQL, Graphene and identity configuration are supplied by the deployment.

Immutable image: `{reference}`.

The `latest` tag follows stable container releases. `image-digest.txt` records
the immutable reference for this build. No frontend packages or documentation
are published by this release workflow.
""")
