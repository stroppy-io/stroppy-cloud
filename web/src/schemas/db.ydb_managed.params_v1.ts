// GENERATED from schemapb schema db.ydb_managed.params@1 — do not edit.
// Yandex Managed YDB: serverless or dedicated database, its presets, scaling and storage.

/** object auto_scale */
export interface DbYdbManagedParams1AutoScale {
  /** Min nodes. scale_policy.auto_scale.min_size. */
  min_size?: number | string;
  /** Max nodes. scale_policy.auto_scale.max_size. */
  max_size?: number | string;
  /** Target CPU. scale_policy.auto_scale.target_tracking.cpu_utilization_percent — the utilization YC scales to hold. [%] */
  cpu_utilization_percent?: number | string;
}

/** root */
export interface DbYdbManagedParams1 {
  /** Database type. serverless = pay-per-request, YC-managed endpoint, no VPC; dedicated = a sized cluster in your network. */
  type: "dedicated" | "serverless";
  /** Availability zone group. YC location the database is created in (location_id on both resources). */
  location_id?: string;
  /** Resource preset. VM configuration of one database node: vCPUs and RAM. */
  resource_preset_id?: string;
  /** Scale policy. fixed = a constant number of nodes; auto = YC scales between min and max on CPU utilization (a preview feature, needs the enable_autoscaling label). */
  scale_policy?: "fixed" | "auto";
  /** Nodes. scale_policy.fixed_scale.size — database slots; production guidance is at least three. */
  node_count?: number | string;
  /** Auto scale. Target-tracking autoscaling bounds; only read when scale_policy is auto. */
  auto_scale?: DbYdbManagedParams1AutoScale;
  /** Storage groups. storage_config.group_count — for the ssd type one group stores up to 100 GB, so size this from the dataset. */
  storage_groups?: number | string;
  /** Storage type. storage_config.storage_type_id; YC documents ssd by example and defers the full list to `yc ydb storage-type list`, so other ids are accepted. */
  storage_type?: string;
  /** Public IPs. Give the dedicated database nodes public addresses; only meaningful for a dedicated database (serverless has no VPC attachment). */
  assign_public_ips?: boolean;
  /** Throttling RCU limit. serverless_database.throttling_rcu_limit — request units per second ceiling; 0 disables throttling. [RU/s] */
  throttling_rcu_limit?: number | string;
  /** Provisioned RCU limit. serverless_database.provisioned_rcu_limit — reserved RU/s billed hourly; 0 disables provisioned capacity. [RU/s] */
  provisioned_rcu_limit?: number | string;
  /** Storage limit. serverless_database.storage_size_limit — the database's data ceiling (YC default 50 GB). [GB] */
  storage_size_limit_gb?: number | string;
  /** Deletion protection. Refuse terraform destroy on the database; a kept stand still tears down its VMs, so leave this off unless the data matters. */
  deletion_protection?: boolean;
}
