-- name: InsertDatabase :exec
INSERT INTO databases (id, tenant_id, name, description, tags, author_id, kind, version, image, params, configs, runtime, external)
VALUES (@id, @tenant_id, @name, @description, @tags, @author_id, @kind, @version, @image, @params, @configs, @runtime, @external);

-- name: DatabaseByID :one
SELECT id, tenant_id, name, description, tags, author_id, kind, version, image, params, configs, runtime, external, created_at, updated_at
FROM databases WHERE id = @id AND deleted_at IS NULL;

-- name: DatabasesOfTenant :many
-- Filters are optional; sort is decided by the caller through sort_key/desc.
-- `topology` sorts by the plan label topology.Compile derives at read time
-- (the UI's topology column), recomputed here from the baked params: they
-- carry every key the kind declares. Keep in step with topology.go labels.
-- `usages` counts the live tests referencing the definition.
SELECT id, tenant_id, name, description, tags, author_id, kind, version, image, params, configs, runtime, external, created_at, updated_at
FROM databases, LATERAL (SELECT CASE kind
    WHEN 'postgres' THEN concat_ws(' + ', 'postgres',
        CASE WHEN COALESCE((params->>'replicas')::numeric::int, 0) = 1 THEN '1 replica' WHEN COALESCE((params->>'replicas')::numeric::int, 0) > 1 THEN (params->>'replicas')::numeric::int || ' replicas' END,
        CASE WHEN params->>'ha' = 'patroni' THEN 'patroni' END,
        CASE WHEN COALESCE((params->>'haproxy')::numeric::int, 0) > 0 THEN 'haproxy' END,
        CASE WHEN params->>'pgbouncer' = 'true' THEN 'pgbouncer' END)
    WHEN 'orioledb' THEN concat_ws(' + ', 'orioledb',
        CASE WHEN COALESCE((params->>'replicas')::numeric::int, 0) = 1 THEN '1 replica' WHEN COALESCE((params->>'replicas')::numeric::int, 0) > 1 THEN (params->>'replicas')::numeric::int || ' replicas' END,
        CASE WHEN params->>'ha' = 'patroni' THEN 'patroni' END,
        CASE WHEN COALESCE((params->>'haproxy')::numeric::int, 0) > 0 THEN 'haproxy' END,
        CASE WHEN params->>'pgbouncer' = 'true' THEN 'pgbouncer' END)
    WHEN 'mysql' THEN concat_ws(' + ', 'mysql',
        CASE WHEN COALESCE((params->>'replicas')::numeric::int, 0) = 1 THEN '1 replica' WHEN COALESCE((params->>'replicas')::numeric::int, 0) > 1 THEN (params->>'replicas')::numeric::int || ' replicas' END,
        CASE WHEN params->>'replication' = 'group' THEN 'group replication' END,
        CASE WHEN COALESCE((params->>'proxysql')::numeric::int, 0) > 0 THEN 'proxysql' END)
    WHEN 'mariadb' THEN CASE WHEN params->>'replication' = 'galera'
        THEN 'galera ×' || COALESCE((params->>'galera_nodes')::numeric::int, 3)
             || CASE WHEN COALESCE((params->>'proxysql')::numeric::int, 0) > 0 THEN ' + proxysql' WHEN params->>'maxscale' = 'true' THEN ' + maxscale' ELSE '' END
        ELSE concat_ws(' + ', 'mariadb',
             CASE WHEN COALESCE((params->>'replicas')::numeric::int, 0) = 1 THEN '1 replica' WHEN COALESCE((params->>'replicas')::numeric::int, 0) > 1 THEN (params->>'replicas')::numeric::int || ' replicas' END,
             CASE WHEN COALESCE((params->>'proxysql')::numeric::int, 0) > 0 OR params->>'maxscale' = 'true' THEN 'proxy' END) END
    WHEN 'picodata' THEN 'picodata ×' || GREATEST(1, COALESCE((SELECT sum(COALESCE((t->>'instances')::numeric::int, 1)) FROM jsonb_array_elements(CASE WHEN jsonb_typeof(params->'tiers') = 'array' THEN params->'tiers' ELSE '[]'::jsonb END) t), 0))
        || CASE WHEN COALESCE((params->>'haproxy')::numeric::int, 0) > 0 THEN ' + haproxy' ELSE '' END
    WHEN 'ydb' THEN 'ydb ' || COALESCE(NULLIF(params->>'fault_tolerance', ''), 'none') || ': ' || COALESCE((params->>'storage_nodes')::numeric::int, 1) || ' storage + ' || COALESCE((params->>'database_nodes')::numeric::int, 1) || ' database'
    WHEN 'cockroach' THEN 'cockroach ×' || COALESCE((params->>'nodes')::numeric::int, 3) || CASE WHEN COALESCE((params->>'haproxy')::numeric::int, 0) > 0 THEN ' + haproxy' ELSE '' END
    WHEN 'pg_noop' THEN 'pg-noop'
    WHEN 'noop' THEN 'noop (runner only)'
    WHEN 'ydb_managed' THEN 'managed ydb ' || COALESCE(NULLIF(params->>'type', ''), 'dedicated')
    WHEN 'external' THEN 'external ' || COALESCE(NULLIF(params->>'protocol', ''), 'pg')
  END AS topology_label) tl
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  AND (@search::text = '' OR name ILIKE '%' || @search::text || '%' OR description ILIKE '%' || @search::text || '%')
  AND (@author_id::text = '' OR author_id::text = @author_id::text)
  AND (COALESCE(cardinality(@kinds::text[]), 0) = 0 OR kind = ANY(@kinds::text[]))
  AND (@tags::jsonb IS NULL OR tags @> @tags::jsonb)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'kind' AND NOT @desc::boolean THEN kind END ASC,
  CASE WHEN @sort_key::text = 'kind' AND @desc::boolean THEN kind END DESC,
  CASE WHEN @sort_key::text = 'version' AND NOT @desc::boolean THEN version END ASC,
  CASE WHEN @sort_key::text = 'version' AND @desc::boolean THEN version END DESC,
  CASE WHEN @sort_key::text = 'topology' AND NOT @desc::boolean THEN tl.topology_label END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'topology' AND @desc::boolean THEN tl.topology_label END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'usages' AND NOT @desc::boolean THEN (SELECT count(*) FROM tests t WHERE t.database_id = databases.id AND t.deleted_at IS NULL) END ASC,
  CASE WHEN @sort_key::text = 'usages' AND @desc::boolean THEN (SELECT count(*) FROM tests t WHERE t.database_id = databases.id AND t.deleted_at IS NULL) END DESC,
  CASE WHEN @sort_key::text = 'author' AND NOT @desc::boolean THEN author_id::text END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND @desc::boolean THEN author_id::text END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_key::text = 'updated_at' AND @desc::boolean THEN updated_at END DESC,
  updated_at DESC, id
LIMIT @lim OFFSET @off;

-- name: UpdateDatabase :execrows
UPDATE databases
SET name        = COALESCE(@name::text, name),
    description = COALESCE(@description::text, description),
    tags        = COALESCE(@tags::jsonb, tags),
    version     = COALESCE(@version::text, version),
    image       = COALESCE(@image::text, image),
    params      = COALESCE(@params::jsonb, params),
    configs     = COALESCE(@configs::jsonb, configs),
    runtime     = COALESCE(@runtime::jsonb, runtime),
    updated_at  = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteDatabase :execrows
UPDATE databases SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: InsertWorkload :exec
INSERT INTO workloads (id, tenant_id, name, description, tags, author_id, stroppy_version, protocol, spec)
VALUES (@id, @tenant_id, @name, @description, @tags, @author_id, @stroppy_version, @protocol, @spec);

-- name: WorkloadByID :one
SELECT id, tenant_id, name, description, tags, author_id, stroppy_version, protocol, spec, created_at, updated_at
FROM workloads WHERE id = @id AND deleted_at IS NULL;

-- name: WorkloadsOfTenant :many
SELECT id, tenant_id, name, description, tags, author_id, stroppy_version, protocol, spec, created_at, updated_at
FROM workloads
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  AND (@search::text = '' OR name ILIKE '%' || @search::text || '%' OR description ILIKE '%' || @search::text || '%')
  AND (@author_id::text = '' OR author_id::text = @author_id::text)
  AND (COALESCE(cardinality(@protocols::text[]), 0) = 0 OR protocol = ANY(@protocols::text[]))
  AND (COALESCE(cardinality(@versions::text[]), 0) = 0 OR stroppy_version = ANY(@versions::text[]))
  AND (@script::text = '' OR spec->'segments' @> jsonb_build_array(jsonb_build_object('workload', jsonb_build_object('script', @script::text))))
  AND (@tags::jsonb IS NULL OR tags @> @tags::jsonb)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'protocol' AND NOT @desc::boolean THEN protocol END ASC,
  CASE WHEN @sort_key::text = 'protocol' AND @desc::boolean THEN protocol END DESC,
  CASE WHEN @sort_key::text = 'stroppy_version' AND NOT @desc::boolean THEN stroppy_version END ASC,
  CASE WHEN @sort_key::text = 'stroppy_version' AND @desc::boolean THEN stroppy_version END DESC,
  CASE WHEN @sort_key::text = 'segments' AND NOT @desc::boolean THEN jsonb_array_length(COALESCE(spec->'segments', '[]'::jsonb)) END ASC,
  CASE WHEN @sort_key::text = 'segments' AND @desc::boolean THEN jsonb_array_length(COALESCE(spec->'segments', '[]'::jsonb)) END DESC,
  CASE WHEN @sort_key::text = 'usages' AND NOT @desc::boolean THEN (SELECT count(*) FROM tests t WHERE t.workload_id = workloads.id AND t.deleted_at IS NULL) END ASC,
  CASE WHEN @sort_key::text = 'usages' AND @desc::boolean THEN (SELECT count(*) FROM tests t WHERE t.workload_id = workloads.id AND t.deleted_at IS NULL) END DESC,
  CASE WHEN @sort_key::text = 'author' AND NOT @desc::boolean THEN author_id::text END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND @desc::boolean THEN author_id::text END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_key::text = 'updated_at' AND @desc::boolean THEN updated_at END DESC,
  updated_at DESC, id
LIMIT @lim OFFSET @off;

-- name: UpdateWorkload :execrows
UPDATE workloads
SET name            = COALESCE(@name::text, name),
    description     = COALESCE(@description::text, description),
    tags            = COALESCE(@tags::jsonb, tags),
    stroppy_version = COALESCE(@stroppy_version::text, stroppy_version),
    protocol        = COALESCE(@protocol::text, protocol),
    spec            = COALESCE(@spec::jsonb, spec),
    updated_at      = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteWorkload :execrows
UPDATE workloads SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: InsertTest :exec
INSERT INTO tests (id, tenant_id, name, description, tags, author_id, database_id, database_inline, workload_id, workload_inline,
                   sizes, execution, provider_profile_id, keep, rating_tenant, rating_global, status, validated_at)
VALUES (@id, @tenant_id, @name, @description, @tags, @author_id, @database_id, @database_inline, @workload_id, @workload_inline,
        @sizes, @execution, @provider_profile_id, @keep, @rating_tenant, @rating_global, @status, @validated_at);

-- name: TestByID :one
SELECT id, tenant_id, name, description, tags, author_id, database_id, database_inline, workload_id, workload_inline,
       sizes, execution, provider_profile_id, keep, rating_tenant, rating_global, status, validated_at, created_at, updated_at
FROM tests WHERE id = @id AND deleted_at IS NULL;

-- name: TestsOfTenant :many
SELECT id, tenant_id, name, description, tags, author_id, database_id, database_inline, workload_id, workload_inline,
       sizes, execution, provider_profile_id, keep, rating_tenant, rating_global, status, validated_at, created_at, updated_at
FROM tests
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  AND (@search::text = '' OR name ILIKE '%' || @search::text || '%' OR description ILIKE '%' || @search::text || '%')
  AND (@author_id::text = '' OR author_id::text = @author_id::text)
  AND (COALESCE(cardinality(@statuses::text[]), 0) = 0 OR status = ANY(@statuses::text[]))
  AND (@tags::jsonb IS NULL OR tags @> @tags::jsonb)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'status' AND NOT @desc::boolean THEN status END ASC,
  CASE WHEN @sort_key::text = 'status' AND @desc::boolean THEN status END DESC,
  CASE WHEN @sort_key::text = 'kind' AND NOT @desc::boolean THEN COALESCE((SELECT d.kind FROM databases d WHERE d.id = tests.database_id), database_inline->>'kind') END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'kind' AND @desc::boolean THEN COALESCE((SELECT d.kind FROM databases d WHERE d.id = tests.database_id), database_inline->>'kind') END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'database' AND NOT @desc::boolean THEN COALESCE((SELECT d.name FROM databases d WHERE d.id = tests.database_id), database_inline->>'name') END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'database' AND @desc::boolean THEN COALESCE((SELECT d.name FROM databases d WHERE d.id = tests.database_id), database_inline->>'name') END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'workload' AND NOT @desc::boolean THEN COALESCE((SELECT w.name FROM workloads w WHERE w.id = tests.workload_id), workload_inline->>'name') END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'workload' AND @desc::boolean THEN COALESCE((SELECT w.name FROM workloads w WHERE w.id = tests.workload_id), workload_inline->>'name') END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'provider' AND NOT @desc::boolean THEN (SELECT pp.name FROM provider_profiles pp WHERE pp.id = tests.provider_profile_id) END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'provider' AND @desc::boolean THEN (SELECT pp.name FROM provider_profiles pp WHERE pp.id = tests.provider_profile_id) END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND NOT @desc::boolean THEN author_id::text END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'author' AND @desc::boolean THEN author_id::text END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'last_run_at' AND NOT @desc::boolean THEN (SELECT max(r.created_at) FROM runs r WHERE r.test_id = tests.id AND r.deleted_at IS NULL) END ASC NULLS LAST,
  CASE WHEN @sort_key::text = 'last_run_at' AND @desc::boolean THEN (SELECT max(r.created_at) FROM runs r WHERE r.test_id = tests.id AND r.deleted_at IS NULL) END DESC NULLS LAST,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_key::text = 'updated_at' AND @desc::boolean THEN updated_at END DESC,
  updated_at DESC, id
LIMIT @lim OFFSET @off;

-- name: UpdateTest :execrows
-- Reference vs inline of a pair switches together: clearing one sets the other.
UPDATE tests
SET name                = COALESCE(@name::text, name),
    description         = COALESCE(@description::text, description),
    tags                = COALESCE(@tags::jsonb, tags),
    database_id         = CASE WHEN @set_database::boolean THEN @database_id::uuid ELSE database_id END,
    database_inline     = CASE WHEN @set_database::boolean THEN @database_inline::jsonb ELSE database_inline END,
    workload_id         = CASE WHEN @set_workload::boolean THEN @workload_id::uuid ELSE workload_id END,
    workload_inline     = CASE WHEN @set_workload::boolean THEN @workload_inline::jsonb ELSE workload_inline END,
    sizes               = COALESCE(@sizes::jsonb, sizes),
    execution           = COALESCE(@execution::jsonb, execution),
    provider_profile_id = CASE WHEN @set_provider::boolean THEN @provider_profile_id::uuid ELSE provider_profile_id END,
    keep                = COALESCE(@keep::text, keep),
    rating_tenant       = COALESCE(@rating_tenant::boolean, rating_tenant),
    rating_global       = COALESCE(@rating_global::boolean, rating_global),
    status              = @status,
    validated_at        = @validated_at,
    updated_at          = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SetTestStatus :exec
UPDATE tests SET status = @status, validated_at = @validated_at WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteTest :execrows
UPDATE tests SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: TestsUsingDatabase :many
SELECT id, name FROM tests WHERE database_id = @database_id AND deleted_at IS NULL ORDER BY name;

-- name: TestsUsingWorkload :many
SELECT id, name FROM tests WHERE workload_id = @workload_id AND deleted_at IS NULL ORDER BY name;

-- name: InlineDatabaseIntoTests :execrows
-- Delete with inline_usages: every referencing test takes a copy.
UPDATE tests SET database_id = NULL, database_inline = @spec, updated_at = now()
WHERE database_id = @database_id AND deleted_at IS NULL;

-- name: InlineWorkloadIntoTests :execrows
UPDATE tests SET workload_id = NULL, workload_inline = @spec, updated_at = now()
WHERE workload_id = @workload_id AND deleted_at IS NULL;
