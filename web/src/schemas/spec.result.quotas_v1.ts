// GENERATED from schemapb schema spec.result.quotas@1 — do not edit.
// Provider quota snapshot: limits and current usage, as observed at a point in time.

/** object  */
export interface SpecResultQuotas1Item {
  /** Quota. Provider quota id, e.g. compute.instanceCores.count. */
  name: string;
  /** Limit. Upper bound the provider enforces. */
  limit: number;
  /** Used. Consumption at observed_at across the quota scope: all folders in the cloud for Yandex Cloud, or the AWS account. */
  used: number;
  /** Unit. Unit of limit and used: count, GB, GiB… */
  unit?: string;
  /** Zone. Set for a zone-scoped quota; empty when the quota is regional or global. */
  zone?: string;
}

/** root */
export interface SpecResultQuotas1 {
  /** Unavailable reason. permission_denied when cloud quotas cannot be read. Quotas are empty and launches rely on provider enforcement instead of the cloud precheck. */
  unavailable_reason?: string;
  /** Scope. Scope of the quota snapshot or denied request, e.g. cloud:b1g... for Yandex Cloud. Cloud quotas cover all folders and zones. */
  scope?: string;
  /** Observed at. When the provider was read; the server treats a snapshot older than a minute as stale. */
  observed_at: string;
  /** Quotas. One entry per quota the provider reports for this profile. */
  quotas: Array<SpecResultQuotas1Item>;
}

