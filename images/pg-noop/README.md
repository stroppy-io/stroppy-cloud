# pg-noop release packaging

This linux/amd64 image packages the unchanged `pgnoop` binary from upstream
release v0.1.2. The Dockerfile verifies the release archive's SHA-256 and pins
the Alpine base digest. Netcat provides the pipeline's TCP health check.

```sh
docker build --platform linux/amd64 -t registry.stroppy.io/stroppy-io/pg-noop:0.1.2-r1 images/pg-noop
```

Upstream supports host, port and worker count. It returns empty PostgreSQL
responses and stores no data. Use `execute_sql` for the protocol smoke test;
report native query/iteration rates. Successful execution does not demonstrate
database transactions or data correctness. Latency/error injection is not
implemented by this upstream release.
