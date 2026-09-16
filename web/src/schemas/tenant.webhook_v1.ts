// GENERATED from schemapb schema tenant.webhook@1 — do not edit.
// Outgoing webhook of a tenant: endpoint, subscribed events and signing secret.

/** root */
export interface TenantWebhook1 {
  /** Endpoint. HTTPS endpoint the event is POSTed to. */
  url: string;
  /** Events. Which events are delivered; at least one. */
  events: Array<"run.started" | "run.stage" | "run.finished" | "run.failed" | "run.cancelled" | "suite.started" | "suite.cell_finished" | "suite.finished">;
  /** Signing secret. Shared secret for the HMAC-SHA256 signature; shown once at creation. */
  secret: string;
  /** Enabled. Deliveries are paused while this is off; the subscription is kept. */
  enabled?: boolean;
  /** Description. What this hook is for, for whoever finds it later. */
  description?: string;
}
