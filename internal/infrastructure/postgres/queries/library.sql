-- name: InsertDatabase :exec
INSERT INTO databases (id, tenant_id, name, description, tags, author_id, kind, version, image, params, configs, runtime, external)
VALUES (@id, @tenant_id, @name, @description, @tags, @author_id, @kind, @version, @image, @params, @configs, @runtime, @external);

-- name: DatabaseByID :one
SELECT id, tenant_id, name, description, tags, author_id, kind, version, image, params, configs, runtime, external, created_at, updated_at
FROM databases WHERE id = @id AND deleted_at IS NULL;

-- name: DatabasesOfTenant :many
-- Filters are optional; sort is decided by the caller through sort_key/desc.
SELECT id, tenant_id, name, description, tags, author_id, kind, version, image, params, configs, runtime, external, created_at, updated_at
FROM databases
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
  AND (@search::text = '' OR name ILIKE '%' || @search::text || '%' OR description ILIKE '%' || @search::text || '%')
  AND (@author_id::text = '' OR author_id::text = @author_id::text)
  AND (cardinality(@kinds::text[]) = 0 OR kind = ANY(@kinds::text[]))
  AND (@tags::jsonb IS NULL OR tags @> @tags::jsonb)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'kind' AND NOT @desc::boolean THEN kind END ASC,
  CASE WHEN @sort_key::text = 'kind' AND @desc::boolean THEN kind END DESC,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
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
  AND (cardinality(@protocols::text[]) = 0 OR protocol = ANY(@protocols::text[]))
  AND (cardinality(@versions::text[]) = 0 OR stroppy_version = ANY(@versions::text[]))
  AND (@script::text = '' OR spec->'segments' @> jsonb_build_array(jsonb_build_object('workload', jsonb_build_object('script', @script::text))))
  AND (@tags::jsonb IS NULL OR tags @> @tags::jsonb)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'protocol' AND NOT @desc::boolean THEN protocol END ASC,
  CASE WHEN @sort_key::text = 'protocol' AND @desc::boolean THEN protocol END DESC,
  CASE WHEN @sort_key::text = 'stroppy_version' AND NOT @desc::boolean THEN stroppy_version END ASC,
  CASE WHEN @sort_key::text = 'stroppy_version' AND @desc::boolean THEN stroppy_version END DESC,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
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
  AND (cardinality(@statuses::text[]) = 0 OR status = ANY(@statuses::text[]))
  AND (@tags::jsonb IS NULL OR tags @> @tags::jsonb)
ORDER BY
  CASE WHEN @sort_key::text = 'name' AND NOT @desc::boolean THEN name END ASC,
  CASE WHEN @sort_key::text = 'name' AND @desc::boolean THEN name END DESC,
  CASE WHEN @sort_key::text = 'created_at' AND NOT @desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_key::text = 'created_at' AND @desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_key::text = 'updated_at' AND NOT @desc::boolean THEN updated_at END ASC,
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
