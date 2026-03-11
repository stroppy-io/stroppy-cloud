-- Create "refresh_tokens" table
CREATE TABLE "public"."refresh_tokens" ("id" text NOT NULL, "session_id" text NULL, "token_hash" text NULL, "revoked" boolean NULL DEFAULT false, "created_at" timestamptz NULL DEFAULT now(), PRIMARY KEY ("id"));
-- Create index "auth_refresh_tokens_session_id_idx" to table: "refresh_tokens"
CREATE INDEX "auth_refresh_tokens_session_id_idx" ON "public"."refresh_tokens" ("session_id");
-- Create index "auth_refresh_tokens_token_hash_idx" to table: "refresh_tokens"
CREATE UNIQUE INDEX "auth_refresh_tokens_token_hash_idx" ON "public"."refresh_tokens" ("token_hash");
-- Create "runs" table
CREATE TABLE "public"."runs" ("id" text NOT NULL, "suite_id" text NULL, "hatchet_run_id" text NULL, "status" integer NULL DEFAULT 0, "test" bytea NULL, "target" integer NULL DEFAULT 0, "created_at" timestamptz NULL DEFAULT now(), "started_at" timestamptz NULL, "finished_at" timestamptz NULL, "duration_ms" bigint NULL, "error_message" text NULL, "dag" bytea NULL, "results" bytea NULL, PRIMARY KEY ("id"));
-- Create index "idx_runs_created_at" to table: "runs"
CREATE INDEX "idx_runs_created_at" ON "public"."runs" ("created_at");
-- Create index "idx_runs_hatchet_run_id" to table: "runs"
CREATE INDEX "idx_runs_hatchet_run_id" ON "public"."runs" ("hatchet_run_id");
-- Create index "idx_runs_status" to table: "runs"
CREATE INDEX "idx_runs_status" ON "public"."runs" ("status");
-- Create index "idx_runs_suite_id" to table: "runs"
CREATE INDEX "idx_runs_suite_id" ON "public"."runs" ("suite_id");
-- Create "sessions" table
CREATE TABLE "public"."sessions" ("id" text NOT NULL, "user_id" text NULL, "created_at" timestamptz NULL DEFAULT now(), "expires_at" timestamptz NULL, PRIMARY KEY ("id"));
-- Create index "auth_sessions_user_id_idx" to table: "sessions"
CREATE INDEX "auth_sessions_user_id_idx" ON "public"."sessions" ("user_id");
-- Create "settings" table
CREATE TABLE "public"."settings" ("id" text NOT NULL DEFAULT 'default', "data" bytea NULL DEFAULT '\x7b7d', "updated_at" timestamptz NULL DEFAULT now(), PRIMARY KEY ("id"));
-- Create "suites" table
CREATE TABLE "public"."suites" ("id" text NOT NULL, "hatchet_run_id" text NULL, "status" integer NULL DEFAULT 0, "test_suite" bytea NULL, "target" integer NULL DEFAULT 0, "created_at" timestamptz NULL DEFAULT now(), "started_at" timestamptz NULL, "finished_at" timestamptz NULL, "duration_ms" bigint NULL, "error_message" text NULL, "results" bytea NULL, PRIMARY KEY ("id"));
-- Create index "idx_suites_created_at" to table: "suites"
CREATE INDEX "idx_suites_created_at" ON "public"."suites" ("created_at");
-- Create index "idx_suites_status" to table: "suites"
CREATE INDEX "idx_suites_status" ON "public"."suites" ("status");
-- Create "users" table
CREATE TABLE "public"."users" ("id" text NOT NULL, "username" text NULL, "encrypted_password" text NULL DEFAULT '', "role" text NULL DEFAULT 'user', "created_at" timestamptz NULL DEFAULT now(), "updated_at" timestamptz NULL DEFAULT now(), PRIMARY KEY ("id"));
-- Create index "auth_users_username_idx" to table: "users"
CREATE UNIQUE INDEX "auth_users_username_idx" ON "public"."users" ("username");
-- Create "workloads" table
CREATE TABLE "public"."workloads" ("id" text NOT NULL, "name" text NULL, "description" text NULL, "builtin" boolean NULL DEFAULT false, "script" bytea NULL, "sql_data" bytea NULL, "probe" bytea NULL, "created_at" timestamptz NULL DEFAULT now(), PRIMARY KEY ("id"));
-- Create index "idx_workloads_name" to table: "workloads"
CREATE UNIQUE INDEX "idx_workloads_name" ON "public"."workloads" ("name");
