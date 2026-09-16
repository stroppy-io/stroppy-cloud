-- Authoritative schema of the stroppy-cloud server. Migrations are diffs of
-- this file (make migrate-generate name=...); generated queries live in
-- gen/db. Keep it readable: one section per aggregate, comments say why.

-- profiles: people, keyed by the IAM subject. Created lazily on the first
-- authenticated call; email mirrors IAM (GET /v1/users/me + webhook
-- email.changed) because the access token deliberately carries none.
CREATE TABLE profiles (
    id            uuid        PRIMARY KEY,
    email         text        NOT NULL DEFAULT '',
    display_name  text        NOT NULL DEFAULT '',
    avatar        text        NOT NULL DEFAULT '',
    is_platform_admin boolean NOT NULL DEFAULT false,
    preferences   jsonb       NOT NULL DEFAULT '{}'::jsonb,
    notifications jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- iam_denylist: sessions revoked by IAM before their access token expires.
-- Hybrid verification does not see revocations, so the webhook lands here
-- and the verifier consults it for the access_ttl window.
CREATE TABLE iam_denylist (
    session_id  text        PRIMARY KEY,
    user_id     uuid        NOT NULL,
    expires_at  timestamptz NOT NULL
);
CREATE INDEX iam_denylist_expires_at_idx ON iam_denylist (expires_at);

-- iam_webhook_events: dedupe of at-least-once webhook delivery by event id.
CREATE TABLE iam_webhook_events (
    id          text        PRIMARY KEY,
    received_at timestamptz NOT NULL DEFAULT now()
);

-- tenants: the organisation. One owned tenant per account; membership in
-- others unlimited. graphene_namespace is t-<slug>, created with the row.
-- Deleting is a soft delete: the namespace retires, the row keeps history.
CREATE TABLE tenants (
    id                 uuid        PRIMARY KEY,
    slug               text        NOT NULL,
    name               text        NOT NULL,
    description        text        NOT NULL DEFAULT '',
    public_name        text,
    status             text        NOT NULL DEFAULT 'active',
    suspended_reason   text        NOT NULL DEFAULT '',
    owner_id           uuid        NOT NULL REFERENCES profiles (id),
    graphene_namespace text        NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz
);
CREATE UNIQUE INDEX tenants_slug_live_idx ON tenants (slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_owner_live_idx ON tenants (owner_id) WHERE deleted_at IS NULL;

-- tenant_members: who is in a tenant with which role (owner|admin|member|viewer).
CREATE TABLE tenant_members (
    tenant_id    uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    role         text        NOT NULL,
    joined_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz,
    PRIMARY KEY (tenant_id, user_id)
);
CREATE INDEX tenant_members_user_idx ON tenant_members (user_id);

-- tenant_invites: by email, 7-day TTL. An invite for an email that has no
-- account yet stays pending and is applied on that email's first login.
CREATE TABLE tenant_invites (
    id          uuid        PRIMARY KEY,
    tenant_id   uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    email       text        NOT NULL,
    role        text        NOT NULL,
    status      text        NOT NULL DEFAULT 'pending',
    invited_by  uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    message     text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    resolved_at timestamptz
);
-- email is stored lowercased by the service, so plain column indexes do.
CREATE UNIQUE INDEX tenant_invites_pending_idx ON tenant_invites (tenant_id, email) WHERE status = 'pending';
CREATE INDEX tenant_invites_email_idx ON tenant_invites (email) WHERE status = 'pending';

-- api_tokens: personal (owned by a person, confined to one tenant, rights =
-- min(token role, owner's current role)) and service (tenant-owned, role
-- member|viewer). Only the sha256 of the secret is stored; the prefix is
-- what people see in lists.
CREATE TABLE api_tokens (
    id           uuid        PRIMARY KEY,
    kind         text        NOT NULL,
    name         text        NOT NULL,
    prefix       text        NOT NULL,
    secret_hash  bytea       NOT NULL,
    tenant_id    uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    role         text        NOT NULL,
    owner_id     uuid        REFERENCES profiles (id) ON DELETE CASCADE,
    expires_at   timestamptz,
    last_used_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    revoked_at   timestamptz
);
CREATE UNIQUE INDEX api_tokens_prefix_idx ON api_tokens (prefix);
CREATE INDEX api_tokens_owner_idx ON api_tokens (owner_id) WHERE revoked_at IS NULL;
CREATE INDEX api_tokens_tenant_idx ON api_tokens (tenant_id) WHERE revoked_at IS NULL;

-- audit_log: who did what to which target. Tenant slice is what tenants
-- see; platform admins see everything.
CREATE TABLE audit_log (
    id          bigserial   PRIMARY KEY,
    at          timestamptz NOT NULL DEFAULT now(),
    tenant_id   uuid,
    actor_kind  text        NOT NULL,
    actor_id    text        NOT NULL DEFAULT '',
    actor_name  text        NOT NULL DEFAULT '',
    action      text        NOT NULL,
    target_kind text        NOT NULL DEFAULT '',
    target_id   text        NOT NULL DEFAULT '',
    target_name text        NOT NULL DEFAULT '',
    details     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    request_id  text        NOT NULL DEFAULT ''
);
CREATE INDEX audit_log_tenant_idx ON audit_log (tenant_id, id DESC);

-- tenant_settings: defaults of a tenant (one row, created lazily).
-- limits_override is the platform admin's per-tenant ceiling (NULL = platform default).
CREATE TABLE tenant_settings (
    tenant_id           uuid        PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    run_retention_days  int         NOT NULL DEFAULT 90,
    rating_tenant       boolean     NOT NULL DEFAULT true,
    rating_global       boolean     NOT NULL DEFAULT false,
    default_keep        text        NOT NULL DEFAULT '0s',
    notification_emails jsonb       NOT NULL DEFAULT '[]'::jsonb,
    limits_override     jsonb,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- provider_profiles: a named cloud account of the tenant. settings is the
-- baked provider.<kind>.settings value; credentials never land here — they
-- go to the Graphene secret named in secret_names. status follows the
-- verification pipeline; quotas is the cache filled by the quota pipeline.
CREATE TABLE provider_profiles (
    id                 uuid        PRIMARY KEY,
    tenant_id          uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name               text        NOT NULL,
    kind               text        NOT NULL,
    settings           jsonb       NOT NULL DEFAULT '{}'::jsonb,
    secret_names       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    status             text        NOT NULL DEFAULT 'verifying',
    status_reason      text        NOT NULL DEFAULT '',
    verified_at        timestamptz,
    verify_run_id      text        NOT NULL DEFAULT '',
    quotas             jsonb,
    quotas_observed_at timestamptz,
    quotas_unavailable_reason text NOT NULL DEFAULT '',
    quotas_scope text NOT NULL DEFAULT '',
    created_by         uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz
);
CREATE UNIQUE INDEX provider_profiles_name_idx ON provider_profiles (tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX provider_profiles_tenant_idx ON provider_profiles (tenant_id) WHERE deleted_at IS NULL;

-- webhooks: outbound Standard Webhooks. The signing secret must be readable
-- to sign, so it is stored as-is; prev_secret keeps the 24h overlap after a
-- rotation.
CREATE TABLE webhooks (
    id               uuid        PRIMARY KEY,
    tenant_id        uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    url              text        NOT NULL,
    events           jsonb       NOT NULL DEFAULT '[]'::jsonb,
    enabled          boolean     NOT NULL DEFAULT true,
    description      text        NOT NULL DEFAULT '',
    secret           text        NOT NULL,
    prev_secret      text        NOT NULL DEFAULT '',
    prev_expires_at  timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhooks_tenant_idx ON webhooks (tenant_id);

-- webhook_deliveries: one row per event per webhook; the dispatcher
-- retries pending rows with backoff until success or the attempt cap.
CREATE TABLE webhook_deliveries (
    id              uuid        PRIMARY KEY,
    webhook_id      uuid        NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event           text        NOT NULL,
    status          text        NOT NULL DEFAULT 'pending',
    attempts        int         NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_attempt_at timestamptz,
    response_status int,
    error           text        NOT NULL DEFAULT '',
    payload         jsonb       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhook_deliveries_webhook_idx ON webhook_deliveries (webhook_id, created_at);
CREATE INDEX webhook_deliveries_due_idx ON webhook_deliveries (next_attempt_at) WHERE status = 'pending';

-- Library: reusable definitions of a tenant. Specs are baked schemapb
-- values (jsonb); derived data (previews, requirements) is computed on
-- read, never stored. Soft delete keeps run snapshots' references valid.
CREATE TABLE databases (
    id          uuid        PRIMARY KEY,
    tenant_id   uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    tags        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    author_id   uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    kind        text        NOT NULL,
    version     text        NOT NULL,
    image       text        NOT NULL DEFAULT '',
    params      jsonb       NOT NULL DEFAULT '{}'::jsonb,
    configs     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    external    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);
CREATE UNIQUE INDEX databases_name_idx ON databases (tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX databases_tenant_idx ON databases (tenant_id, updated_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE workloads (
    id              uuid        PRIMARY KEY,
    tenant_id       uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name            text        NOT NULL,
    description     text        NOT NULL DEFAULT '',
    tags            jsonb       NOT NULL DEFAULT '{}'::jsonb,
    author_id       uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    stroppy_version text        NOT NULL,
    protocol        text        NOT NULL,
    spec            jsonb       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);
CREATE UNIQUE INDEX workloads_name_idx ON workloads (tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX workloads_tenant_idx ON workloads (tenant_id, updated_at DESC) WHERE deleted_at IS NULL;

-- tests: database/workload by reference (uuid) or inline (jsonb spec);
-- exactly one of each pair is set once the draft is complete.
CREATE TABLE tests (
    id                  uuid        PRIMARY KEY,
    tenant_id           uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name                text        NOT NULL,
    description         text        NOT NULL DEFAULT '',
    tags                jsonb       NOT NULL DEFAULT '{}'::jsonb,
    author_id           uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    database_id         uuid        REFERENCES databases (id),
    database_inline     jsonb,
    workload_id         uuid        REFERENCES workloads (id),
    workload_inline     jsonb,
    sizes               jsonb       NOT NULL DEFAULT '{}'::jsonb,
    provider_profile_id uuid        REFERENCES provider_profiles (id),
    keep                text        NOT NULL DEFAULT '',
    rating_tenant       boolean     NOT NULL DEFAULT true,
    rating_global       boolean     NOT NULL DEFAULT false,
    status              text        NOT NULL DEFAULT 'draft',
    validated_at        timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);
CREATE UNIQUE INDEX tests_name_idx ON tests (tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX tests_tenant_idx ON tests (tenant_id, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX tests_database_idx ON tests (database_id) WHERE deleted_at IS NULL;
CREATE INDEX tests_workload_idx ON tests (workload_id) WHERE deleted_at IS NULL;

-- runs: one launched RunSpec. The snapshot (database/workload/sizes/
-- provider) and the baked run_spec are frozen at launch; runtime_state is
-- the projection of Graphene events the UI reads; result is spec.result.run.
CREATE TABLE runs (
    id                  uuid        PRIMARY KEY,
    tenant_id           uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name                text        NOT NULL,
    status              text        NOT NULL DEFAULT 'pending',
    phase               text        NOT NULL DEFAULT 'queued',
    status_reason       text        NOT NULL DEFAULT '',
    trigger             text        NOT NULL DEFAULT 'manual',
    suite_run_id        uuid,
    cell_id             text        NOT NULL DEFAULT '',
    schedule_id         uuid,
    parent_run_id       uuid,
    test_id             uuid        REFERENCES tests (id),
    test_name           text        NOT NULL DEFAULT '',
    author_id           uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    snapshot            jsonb       NOT NULL,
    run_spec            jsonb       NOT NULL,
    summary             jsonb       NOT NULL DEFAULT '{}'::jsonb,
    result              jsonb,
    runtime_state       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    last_event_id       bigint      NOT NULL DEFAULT 0,
    rating_tenant       boolean     NOT NULL DEFAULT true,
    rating_global       boolean     NOT NULL DEFAULT false,
    keep                text        NOT NULL DEFAULT '',
    keep_until          timestamptz,
    stand_kept          boolean     NOT NULL DEFAULT false,
    notes               text        NOT NULL DEFAULT '',
    labels              jsonb       NOT NULL DEFAULT '{}'::jsonb,
    graphene_namespace  text        NOT NULL,
    -- graphene_run_id is the run id on the Graphene side; '' means the
    -- row id. Suite cells get "<suite_run_id>-<cell_id>" (SDK RunAll).
    graphene_run_id     text        NOT NULL DEFAULT '',
    pipeline_revision   text        NOT NULL DEFAULT '',
    tps                 double precision,
    duration_seconds    double precision,
    idempotency_key     text        NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    started_at          timestamptz,
    finished_at         timestamptz,
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);
CREATE INDEX runs_tenant_idx ON runs (tenant_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX runs_test_idx ON runs (test_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX runs_live_idx ON runs (status) WHERE deleted_at IS NULL AND status IN ('pending', 'running', 'cancelling');
CREATE UNIQUE INDEX runs_idempotency_idx ON runs (tenant_id, idempotency_key) WHERE idempotency_key <> '';

-- run_events: the timeline the server keeps (milestones from the pipeline
-- and status transitions), keyed by the Graphene event id for resume.
CREATE TABLE run_events (
    id          bigserial   PRIMARY KEY,
    run_id      uuid        NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    graphene_id bigint      NOT NULL DEFAULT 0,
    at          timestamptz NOT NULL,
    kind        text        NOT NULL,
    title       text        NOT NULL,
    subject     text        NOT NULL DEFAULT '',
    status      text        NOT NULL DEFAULT '',
    error       text        NOT NULL DEFAULT '',
    attempt     int         NOT NULL DEFAULT 0,
    payload     jsonb       NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX run_events_run_idx ON run_events (run_id, id);
CREATE UNIQUE INDEX run_events_graphene_idx ON run_events (run_id, graphene_id) WHERE graphene_id <> 0;

-- favorites: per user, per kind of thing.
CREATE TABLE favorites (
    user_id    uuid        NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    tenant_id  uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    kind       text        NOT NULL,
    target_id  uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, kind, target_id)
);
CREATE INDEX favorites_tenant_idx ON favorites (tenant_id, user_id, kind);

-- suites: a matrix of tests × axes with the materialised cells (§16.7).
CREATE TABLE suites (
    id          uuid        PRIMARY KEY,
    tenant_id   uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    tags        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    author_id   uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    tests       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    axes        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    cells       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    concurrency int         NOT NULL DEFAULT 1,
    defaults    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);
CREATE UNIQUE INDEX suites_name_idx ON suites (tenant_id, name) WHERE deleted_at IS NULL;

-- suite_runs: one execution of a suite; the child runs are rows of `runs`
-- with suite_run_id / cell_id.
CREATE TABLE suite_runs (
    id                 uuid        PRIMARY KEY,
    tenant_id          uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    suite_id           uuid        REFERENCES suites (id),
    suite_name         text        NOT NULL DEFAULT '',
    name               text        NOT NULL,
    status             text        NOT NULL DEFAULT 'pending',
    status_reason      text        NOT NULL DEFAULT '',
    trigger            text        NOT NULL DEFAULT 'manual',
    schedule_id        uuid,
    retry_of           uuid,
    concurrency        int         NOT NULL DEFAULT 1,
    cells              jsonb       NOT NULL DEFAULT '[]'::jsonb,
    labels             jsonb       NOT NULL DEFAULT '{}'::jsonb,
    author_id          uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    graphene_namespace text        NOT NULL,
    last_event_id      bigint      NOT NULL DEFAULT 0,
    idempotency_key    text        NOT NULL DEFAULT '',
    created_at         timestamptz NOT NULL DEFAULT now(),
    started_at         timestamptz,
    finished_at        timestamptz,
    duration_seconds   double precision,
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz
);
CREATE INDEX suite_runs_tenant_idx ON suite_runs (tenant_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX suite_runs_suite_idx ON suite_runs (suite_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX suite_runs_idempotency_idx ON suite_runs (tenant_id, idempotency_key) WHERE idempotency_key <> '';

-- schedules: cron on a test or a suite, executed by the server (§16.7).
CREATE TABLE schedules (
    id          uuid        PRIMARY KEY,
    tenant_id   uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        text        NOT NULL,
    target_kind text        NOT NULL,
    target_id   uuid        NOT NULL,
    target_name text        NOT NULL DEFAULT '',
    cron        text        NOT NULL,
    timezone    text        NOT NULL DEFAULT 'UTC',
    enabled     boolean     NOT NULL DEFAULT true,
    overrides   jsonb       NOT NULL DEFAULT '{}'::jsonb,
    next_run_at timestamptz,
    last_run    jsonb,
    author_id   uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);
CREATE INDEX schedules_tenant_idx ON schedules (tenant_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX schedules_due_idx ON schedules (next_run_at) WHERE deleted_at IS NULL AND enabled;

-- schedule_history: what every firing launched.
CREATE TABLE schedule_history (
    id          bigserial   PRIMARY KEY,
    schedule_id uuid        NOT NULL REFERENCES schedules (id) ON DELETE CASCADE,
    kind        text        NOT NULL,
    ref_id      uuid,
    name        text        NOT NULL DEFAULT '',
    status      text        NOT NULL DEFAULT '',
    error       text        NOT NULL DEFAULT '',
    at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX schedule_history_idx ON schedule_history (schedule_id, at DESC);

-- shares: public snapshots of a run, a suite run or a comparison (§16.8).
-- The snapshot is an allowlisted projection captured at share time
-- (secret fields masked); rebuild re-captures it.
CREATE TABLE shares (
    id          uuid        PRIMARY KEY,
    tenant_id   uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    token       text        NOT NULL,
    target_kind text        NOT NULL,
    target_id   uuid,
    target_name text        NOT NULL DEFAULT '',
    run_ids     jsonb       NOT NULL DEFAULT '[]'::jsonb,
    scope       text        NOT NULL DEFAULT 'overview',
    title       text        NOT NULL DEFAULT '',
    snapshot    jsonb       NOT NULL,
    captured_at timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz,
    revoked_at  timestamptz,
    view_count  int         NOT NULL DEFAULT 0,
    created_by  uuid        REFERENCES profiles (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX shares_token_idx ON shares (token);
CREATE INDEX shares_tenant_idx ON shares (tenant_id, created_at DESC);
CREATE INDEX shares_target_idx ON shares (target_kind, target_id);

-- system_settings: the platform's single row (§16.9): tenant creation
-- policy, public rating, examples, default limits of new tenants, the
-- stroppy catalog override.
CREATE TABLE system_settings (
    id                     int         PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    tenant_creation        text        NOT NULL DEFAULT 'anyone',
    public_rating_enabled  boolean     NOT NULL DEFAULT true,
    examples_enabled       boolean     NOT NULL DEFAULT true,
    default_limits         jsonb,
    run_retention_max_days int         NOT NULL DEFAULT 365,
    stroppy_catalog        jsonb,
    updated_at             timestamptz NOT NULL DEFAULT now(),
    updated_by             uuid        REFERENCES profiles (id) ON DELETE SET NULL
);

-- pipeline_pushes: the push state of the pipeline binaries per Graphene
-- namespace (§7): one server build = one revision; a tenant is behind
-- until its namespace carries the running server's revision.
CREATE TABLE pipeline_pushes (
    namespace  text        PRIMARY KEY,
    revision   text        NOT NULL DEFAULT '',
    status     text        NOT NULL DEFAULT 'pending',
    error      text        NOT NULL DEFAULT '',
    pushed_at  timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);
