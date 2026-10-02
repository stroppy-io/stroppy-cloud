CREATE SEQUENCE "public"."audit_log_id_seq" AS int8 INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START WITH 1 CACHE 1;
CREATE SEQUENCE "public"."run_events_id_seq" AS int8 INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START WITH 1 CACHE 1;
CREATE SEQUENCE "public"."schedule_history_id_seq" AS int8 INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START WITH 1 CACHE 1;
CREATE TABLE "public"."api_tokens" (
  "id" uuid NOT NULL,
  "kind" text NOT NULL,
  "name" text NOT NULL,
  "prefix" text NOT NULL,
  "secret_hash" bytea NOT NULL,
  "tenant_id" uuid NOT NULL,
  "role" text NOT NULL,
  "owner_id" uuid,
  "expires_at" timestamptz,
  "last_used_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "revoked_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."audit_log" (
  "id" int8 NOT NULL DEFAULT nextval('audit_log_id_seq'::regclass),
  "at" timestamptz NOT NULL DEFAULT now(),
  "tenant_id" uuid,
  "actor_kind" text NOT NULL,
  "actor_id" text NOT NULL DEFAULT ''::text,
  "actor_name" text NOT NULL DEFAULT ''::text,
  "action" text NOT NULL,
  "target_kind" text NOT NULL DEFAULT ''::text,
  "target_id" text NOT NULL DEFAULT ''::text,
  "target_name" text NOT NULL DEFAULT ''::text,
  "details" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "request_id" text NOT NULL DEFAULT ''::text,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."databases" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "tags" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "kind" text NOT NULL,
  "version" text NOT NULL,
  "image" text NOT NULL DEFAULT ''::text,
  "params" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "configs" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "runtime" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "external" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."favorites" (
  "user_id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "target_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("user_id", "kind", "target_id")
);
CREATE TABLE "public"."pipeline_pushes" (
  "namespace" text NOT NULL,
  "revision" text NOT NULL DEFAULT ''::text,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "error" text NOT NULL DEFAULT ''::text,
  "pushed_at" timestamptz,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("namespace")
);
CREATE TABLE "public"."profiles" (
  "id" uuid NOT NULL,
  "email" text NOT NULL DEFAULT ''::text,
  "display_name" text NOT NULL DEFAULT ''::text,
  "avatar" text NOT NULL DEFAULT ''::text,
  "is_platform_admin" bool NOT NULL DEFAULT false,
  "preferences" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "notifications" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."provider_profiles" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "kind" text NOT NULL,
  "settings" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "secret_names" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "status" text NOT NULL DEFAULT 'verifying'::text,
  "status_reason" text NOT NULL DEFAULT ''::text,
  "verified_at" timestamptz,
  "verify_run_id" text NOT NULL DEFAULT ''::text,
  "quotas" jsonb,
  "quotas_observed_at" timestamptz,
  "quotas_unavailable_reason" text NOT NULL DEFAULT ''::text,
  "quotas_scope" text NOT NULL DEFAULT ''::text,
  "created_by" uuid,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."run_events" (
  "id" int8 NOT NULL DEFAULT nextval('run_events_id_seq'::regclass),
  "run_id" uuid NOT NULL,
  "graphene_id" int8 NOT NULL DEFAULT 0,
  "at" timestamptz NOT NULL,
  "kind" text NOT NULL,
  "title" text NOT NULL,
  "subject" text NOT NULL DEFAULT ''::text,
  "status" text NOT NULL DEFAULT ''::text,
  "error" text NOT NULL DEFAULT ''::text,
  "attempt" int4 NOT NULL DEFAULT 0,
  "payload" jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."runs" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "phase" text NOT NULL DEFAULT 'queued'::text,
  "status_reason" text NOT NULL DEFAULT ''::text,
  "trigger" text NOT NULL DEFAULT 'manual'::text,
  "suite_run_id" uuid,
  "cell_id" text NOT NULL DEFAULT ''::text,
  "schedule_id" uuid,
  "parent_run_id" uuid,
  "test_id" uuid,
  "test_name" text NOT NULL DEFAULT ''::text,
  "author_id" uuid,
  "snapshot" jsonb NOT NULL,
  "run_spec" jsonb NOT NULL,
  "summary" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "result" jsonb,
  "runtime_state" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "last_event_id" int8 NOT NULL DEFAULT 0,
  "rating_tenant" bool NOT NULL DEFAULT true,
  "rating_global" bool NOT NULL DEFAULT false,
  "keep" text NOT NULL DEFAULT ''::text,
  "keep_until" timestamptz,
  "stand_kept" bool NOT NULL DEFAULT false,
  "notes" text NOT NULL DEFAULT ''::text,
  "labels" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "graphene_namespace" text NOT NULL,
  "graphene_run_id" text NOT NULL DEFAULT ''::text,
  "pipeline_revision" text NOT NULL DEFAULT ''::text,
  "tps" float8,
  "duration_seconds" float8,
  "idempotency_key" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "started_at" timestamptz,
  "finished_at" timestamptz,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."schedule_history" (
  "id" int8 NOT NULL DEFAULT nextval('schedule_history_id_seq'::regclass),
  "schedule_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "ref_id" uuid,
  "name" text NOT NULL DEFAULT ''::text,
  "status" text NOT NULL DEFAULT ''::text,
  "error" text NOT NULL DEFAULT ''::text,
  "at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."schedules" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "target_kind" text NOT NULL,
  "target_id" uuid NOT NULL,
  "target_name" text NOT NULL DEFAULT ''::text,
  "cron" text NOT NULL,
  "timezone" text NOT NULL DEFAULT 'UTC'::text,
  "enabled" bool NOT NULL DEFAULT true,
  "overrides" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "next_run_at" timestamptz,
  "last_run" jsonb,
  "author_id" uuid,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."shares" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "token" text NOT NULL,
  "target_kind" text NOT NULL,
  "target_id" uuid,
  "target_name" text NOT NULL DEFAULT ''::text,
  "run_ids" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "scope" text NOT NULL DEFAULT 'overview'::text,
  "title" text NOT NULL DEFAULT ''::text,
  "snapshot" jsonb NOT NULL,
  "captured_at" timestamptz NOT NULL DEFAULT now(),
  "expires_at" timestamptz,
  "revoked_at" timestamptz,
  "view_count" int4 NOT NULL DEFAULT 0,
  "created_by" uuid,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."suite_runs" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "suite_id" uuid,
  "suite_name" text NOT NULL DEFAULT ''::text,
  "name" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "status_reason" text NOT NULL DEFAULT ''::text,
  "trigger" text NOT NULL DEFAULT 'manual'::text,
  "schedule_id" uuid,
  "retry_of" uuid,
  "concurrency" int4 NOT NULL DEFAULT 1,
  "cells" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "labels" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "graphene_namespace" text NOT NULL,
  "last_event_id" int8 NOT NULL DEFAULT 0,
  "idempotency_key" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "started_at" timestamptz,
  "finished_at" timestamptz,
  "duration_seconds" float8,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."suites" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "tags" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "tests" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "axes" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "cells" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "concurrency" int4 NOT NULL DEFAULT 1,
  "defaults" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."system_settings" (
  "id" int4 NOT NULL DEFAULT 1,
  "tenant_creation" text NOT NULL DEFAULT 'anyone'::text,
  "public_rating_enabled" bool NOT NULL DEFAULT true,
  "examples_enabled" bool NOT NULL DEFAULT true,
  "default_limits" jsonb,
  "run_retention_max_days" int4 NOT NULL DEFAULT 365,
  "stroppy_catalog" jsonb,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "updated_by" uuid,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."tenant_invites" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "email" text NOT NULL,
  "role" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "invited_by" uuid,
  "message" text NOT NULL DEFAULT ''::text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "expires_at" timestamptz NOT NULL,
  "resolved_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."tenant_members" (
  "tenant_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "role" text NOT NULL,
  "joined_at" timestamptz NOT NULL DEFAULT now(),
  "last_seen_at" timestamptz,
  PRIMARY KEY ("tenant_id", "user_id")
);
CREATE TABLE "public"."tenant_settings" (
  "tenant_id" uuid NOT NULL,
  "run_retention_days" int4 NOT NULL DEFAULT 90,
  "rating_tenant" bool NOT NULL DEFAULT true,
  "rating_global" bool NOT NULL DEFAULT false,
  "default_keep" text NOT NULL DEFAULT '0s'::text,
  "notification_emails" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "limits_override" jsonb,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id")
);
CREATE TABLE "public"."tenants" (
  "retiring" bool NOT NULL DEFAULT false,
  "id" uuid NOT NULL,
  "slug" text NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "public_name" text,
  "status" text NOT NULL DEFAULT 'active'::text,
  "suspended_reason" text NOT NULL DEFAULT ''::text,
  "owner_id" uuid NOT NULL,
  "graphene_namespace" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."tests" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "tags" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "database_id" uuid,
  "database_inline" jsonb,
  "workload_id" uuid,
  "workload_inline" jsonb,
  "sizes" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "execution" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "provider_profile_id" uuid,
  "keep" text NOT NULL DEFAULT ''::text,
  "rating_tenant" bool NOT NULL DEFAULT true,
  "rating_global" bool NOT NULL DEFAULT false,
  "status" text NOT NULL DEFAULT 'draft'::text,
  "validated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."webhook_deliveries" (
  "id" uuid NOT NULL,
  "webhook_id" uuid NOT NULL,
  "event" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "attempts" int4 NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "last_attempt_at" timestamptz,
  "response_status" int4,
  "error" text NOT NULL DEFAULT ''::text,
  "payload" jsonb NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."webhooks" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "url" text NOT NULL,
  "events" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "enabled" bool NOT NULL DEFAULT true,
  "description" text NOT NULL DEFAULT ''::text,
  "secret" text NOT NULL,
  "prev_secret" text NOT NULL DEFAULT ''::text,
  "prev_expires_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."workloads" (
  "id" uuid NOT NULL,
  "tenant_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT ''::text,
  "tags" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "author_id" uuid,
  "stroppy_version" text NOT NULL,
  "protocol" text NOT NULL,
  "spec" jsonb NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "deleted_at" timestamptz,
  PRIMARY KEY ("id")
);
ALTER TABLE "public"."system_settings" ADD CONSTRAINT "system_settings_id_check" CHECK (id = 1);
CREATE INDEX "api_tokens_owner_idx" ON "public"."api_tokens" ("owner_id") WHERE (revoked_at IS NULL);
CREATE UNIQUE INDEX "api_tokens_prefix_idx" ON "public"."api_tokens" ("prefix");
CREATE INDEX "api_tokens_tenant_idx" ON "public"."api_tokens" ("tenant_id") WHERE (revoked_at IS NULL);
CREATE INDEX "audit_log_tenant_idx" ON "public"."audit_log" ("tenant_id", "id" DESC);
CREATE UNIQUE INDEX "databases_name_idx" ON "public"."databases" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "databases_tenant_idx" ON "public"."databases" ("tenant_id", "updated_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "favorites_tenant_idx" ON "public"."favorites" ("tenant_id", "user_id", "kind");
CREATE UNIQUE INDEX "provider_profiles_name_idx" ON "public"."provider_profiles" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "provider_profiles_tenant_idx" ON "public"."provider_profiles" ("tenant_id") WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "run_events_graphene_idx" ON "public"."run_events" ("run_id", "graphene_id") WHERE (graphene_id <> 0);
CREATE INDEX "run_events_run_idx" ON "public"."run_events" ("run_id", "id");
CREATE UNIQUE INDEX "runs_idempotency_idx" ON "public"."runs" ("tenant_id", "idempotency_key") WHERE (idempotency_key <> ''::text);
CREATE INDEX "runs_live_idx" ON "public"."runs" ("status") WHERE ((deleted_at IS NULL) AND (status = ANY (ARRAY['pending'::text, 'running'::text, 'cancelling'::text])));
CREATE INDEX "runs_tenant_idx" ON "public"."runs" ("tenant_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "runs_test_idx" ON "public"."runs" ("test_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "schedule_history_idx" ON "public"."schedule_history" ("schedule_id", "at" DESC);
CREATE INDEX "schedules_due_idx" ON "public"."schedules" ("next_run_at") WHERE ((deleted_at IS NULL) AND enabled);
CREATE INDEX "schedules_tenant_idx" ON "public"."schedules" ("tenant_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "shares_target_idx" ON "public"."shares" ("target_kind", "target_id");
CREATE INDEX "shares_tenant_idx" ON "public"."shares" ("tenant_id", "created_at" DESC);
CREATE UNIQUE INDEX "shares_token_idx" ON "public"."shares" ("token");
CREATE UNIQUE INDEX "suite_runs_idempotency_idx" ON "public"."suite_runs" ("tenant_id", "idempotency_key") WHERE (idempotency_key <> ''::text);
CREATE INDEX "suite_runs_suite_idx" ON "public"."suite_runs" ("suite_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "suite_runs_tenant_idx" ON "public"."suite_runs" ("tenant_id", "created_at" DESC) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "suites_name_idx" ON "public"."suites" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "tenant_invites_email_idx" ON "public"."tenant_invites" ("email") WHERE (status = 'pending'::text);
CREATE UNIQUE INDEX "tenant_invites_pending_idx" ON "public"."tenant_invites" ("tenant_id", "email") WHERE (status = 'pending'::text);
CREATE INDEX "tenant_members_user_idx" ON "public"."tenant_members" ("user_id");
CREATE UNIQUE INDEX "tenants_owner_live_idx" ON "public"."tenants" ("owner_id") WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "tenants_slug_live_idx" ON "public"."tenants" ("slug") WHERE (deleted_at IS NULL);
CREATE INDEX "tests_database_idx" ON "public"."tests" ("database_id") WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX "tests_name_idx" ON "public"."tests" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "tests_tenant_idx" ON "public"."tests" ("tenant_id", "updated_at" DESC) WHERE (deleted_at IS NULL);
CREATE INDEX "tests_workload_idx" ON "public"."tests" ("workload_id") WHERE (deleted_at IS NULL);
CREATE INDEX "webhook_deliveries_due_idx" ON "public"."webhook_deliveries" ("next_attempt_at") WHERE (status = 'pending'::text);
CREATE INDEX "webhook_deliveries_webhook_idx" ON "public"."webhook_deliveries" ("webhook_id", "created_at");
CREATE INDEX "webhooks_tenant_idx" ON "public"."webhooks" ("tenant_id");
CREATE UNIQUE INDEX "workloads_name_idx" ON "public"."workloads" ("tenant_id", "name") WHERE (deleted_at IS NULL);
CREATE INDEX "workloads_tenant_idx" ON "public"."workloads" ("tenant_id", "updated_at" DESC) WHERE (deleted_at IS NULL);
ALTER TABLE "public"."api_tokens" ADD CONSTRAINT "api_tokens_owner_id_fkey" FOREIGN KEY ("owner_id") REFERENCES "public"."profiles" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."api_tokens" ADD CONSTRAINT "api_tokens_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."databases" ADD CONSTRAINT "databases_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."databases" ADD CONSTRAINT "databases_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."favorites" ADD CONSTRAINT "favorites_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."favorites" ADD CONSTRAINT "favorites_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."profiles" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."provider_profiles" ADD CONSTRAINT "provider_profiles_created_by_fkey" FOREIGN KEY ("created_by") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."provider_profiles" ADD CONSTRAINT "provider_profiles_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."run_events" ADD CONSTRAINT "run_events_run_id_fkey" FOREIGN KEY ("run_id") REFERENCES "public"."runs" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."runs" ADD CONSTRAINT "runs_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."runs" ADD CONSTRAINT "runs_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."runs" ADD CONSTRAINT "runs_test_id_fkey" FOREIGN KEY ("test_id") REFERENCES "public"."tests" ("id");
ALTER TABLE "public"."schedule_history" ADD CONSTRAINT "schedule_history_schedule_id_fkey" FOREIGN KEY ("schedule_id") REFERENCES "public"."schedules" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."schedules" ADD CONSTRAINT "schedules_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."schedules" ADD CONSTRAINT "schedules_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."shares" ADD CONSTRAINT "shares_created_by_fkey" FOREIGN KEY ("created_by") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."shares" ADD CONSTRAINT "shares_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."suite_runs" ADD CONSTRAINT "suite_runs_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."suite_runs" ADD CONSTRAINT "suite_runs_suite_id_fkey" FOREIGN KEY ("suite_id") REFERENCES "public"."suites" ("id");
ALTER TABLE "public"."suite_runs" ADD CONSTRAINT "suite_runs_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."suites" ADD CONSTRAINT "suites_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."suites" ADD CONSTRAINT "suites_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."system_settings" ADD CONSTRAINT "system_settings_updated_by_fkey" FOREIGN KEY ("updated_by") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."tenant_invites" ADD CONSTRAINT "tenant_invites_invited_by_fkey" FOREIGN KEY ("invited_by") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."tenant_invites" ADD CONSTRAINT "tenant_invites_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenant_members" ADD CONSTRAINT "tenant_members_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenant_members" ADD CONSTRAINT "tenant_members_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."profiles" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenant_settings" ADD CONSTRAINT "tenant_settings_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tenants" ADD CONSTRAINT "tenants_owner_id_fkey" FOREIGN KEY ("owner_id") REFERENCES "public"."profiles" ("id");
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_database_id_fkey" FOREIGN KEY ("database_id") REFERENCES "public"."databases" ("id");
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_provider_profile_id_fkey" FOREIGN KEY ("provider_profile_id") REFERENCES "public"."provider_profiles" ("id");
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."tests" ADD CONSTRAINT "tests_workload_id_fkey" FOREIGN KEY ("workload_id") REFERENCES "public"."workloads" ("id");
ALTER TABLE "public"."webhook_deliveries" ADD CONSTRAINT "webhook_deliveries_webhook_id_fkey" FOREIGN KEY ("webhook_id") REFERENCES "public"."webhooks" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."webhooks" ADD CONSTRAINT "webhooks_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;
ALTER TABLE "public"."workloads" ADD CONSTRAINT "workloads_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."profiles" ("id") ON DELETE SET NULL;
ALTER TABLE "public"."workloads" ADD CONSTRAINT "workloads_tenant_id_fkey" FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants" ("id") ON DELETE CASCADE;

