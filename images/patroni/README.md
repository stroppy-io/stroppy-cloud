# PostgreSQL with Patroni

The image combines the official PostgreSQL image with Patroni 4.1.5 and
psycopg2 2.9.13. PostgreSQL versions 15, 16, 17 and 18 are built separately.
The image entrypoint prepares the mounted data directory and starts Patroni as
the postgres user. It does not run the official PostgreSQL initialization
entrypoint: Patroni owns initdb, replication and promotion.

Build from the repository root (example for PostgreSQL 17):

```sh
docker build --build-arg PG_MAJOR=17 -t registry.stroppy.io/stroppy-io/patroni:pg17-4.1.5 images/patroni
docker push registry.stroppy.io/stroppy-io/patroni:pg17-4.1.5
```

Use existing registry credentials. The run pulls through the anonymous Nexus
group at `docker.stroppy.io/stroppy-io/patroni:pg17-4.1.5`.
Published digests and runtime versions are recorded in
[patroni-images.json](../../pipelines/live/patroni-images.json).
Rebuilding a base major tag can select a newer PostgreSQL patch release;
the recorded digest identifies the bytes used by the verification.

The compiler mounts version-specific PostgreSQL configuration as `custom_conf`
and uses `hba_file` for the externally rendered HBA file. HAProxy checks the
Patroni `/primary` and `/replica` endpoints. These settings follow the upstream
[YAML reference](https://patroni.readthedocs.io/en/latest/yaml_configuration.html)
and [REST API](https://patroni.readthedocs.io/en/latest/rest_api.html).
