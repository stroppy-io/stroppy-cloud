-- sqld:up
ALTER TABLE "public"."databases" ADD COLUMN "topology_label" text NOT NULL DEFAULT ''::text;

-- sqld:down
ALTER TABLE "public"."databases" DROP COLUMN "topology_label";
