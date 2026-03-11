-- Create "topology_templates" table
CREATE TABLE "public"."topology_templates" ("id" text NOT NULL, "name" text NULL, "description" text NULL, "database_type" integer NULL DEFAULT 0, "builtin" boolean NULL DEFAULT false, "template_data" bytea NULL, "created_at" timestamptz NULL DEFAULT now(), "updated_at" timestamptz NULL DEFAULT now(), PRIMARY KEY ("id"));
-- Create index "idx_topology_templates_database_type" to table: "topology_templates"
CREATE INDEX "idx_topology_templates_database_type" ON "public"."topology_templates" ("database_type");
-- Create index "idx_topology_templates_name" to table: "topology_templates"
CREATE UNIQUE INDEX "idx_topology_templates_name" ON "public"."topology_templates" ("name");
