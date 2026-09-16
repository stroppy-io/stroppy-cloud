// GENERATED from schemapb schema db.ydb.params@1 — do not edit.
// Self-hosted YDB topology: storage/database nodes, erasure mode, pdisks, database path.

/** root */
export interface DbYdbParams1 {
  /** YDB version. ydbd server series; binaries are fetched through the stroppy gateway. */
  version: "26.3" | "26.2" | "26.1" | "25.4";
  /** Erasure mode. config.erasure: none = no redundancy (dev only); block-4-2 = 4 data + 2 parity over >=8 fail domains; mirror-3-dc = 3 realms x 3 domains, >=9 nodes. */
  fault_tolerance: "none" | "block-4-2" | "mirror-3-dc";
  /** Fail domain level. config.fail_domain_type: which level of host.location the storage layer treats as one failure unit. Upstream uses disk for single-host block-4-2 and rack for mirror-3-dc. */
  failure_domain: "disk" | "body" | "rack";
  /** Storage nodes. Nodes running the BlobStorage layer; together with pdisks_per_node they supply the fail domains the erasure mode demands. */
  storage_nodes?: number | string;
  /** Database nodes. Nodes running the query/tablet layer of the database; these are what the workload connects to. */
  database_nodes?: number | string;
  /** PDisks per storage node. Drive entries per host_config; with failure_domain=disk each pdisk is its own fail domain, which is how a small stand reaches 8 or 9 of them. */
  pdisks_per_node?: number | string;
  /** PDisk device type. config.default_disk_type and the drive type of every host_config entry; storage pools are built per device type. */
  disk_type: "SSD" | "NVME" | "HDD";
  /** Storage groups. Blob storage groups created in the database's storage pool; more groups spread tablets wider. */
  storage_groups?: number | string;
  /** Auto-size pdisks. Let the recipe size the pdisk files from the machine's free disk instead of a fixed size. */
  auto_size_pdisks?: boolean;
  /** Database path. Full path of the benchmark database inside the cluster; /Root itself is reserved for the cluster root. */
  database_path?: string;
  /** TLS (grpcs). Serve the gRPC endpoint over TLS (grpcs://) with a self-signed cluster certificate instead of plain grpc://. */
  grpcs?: boolean;
  /** HAProxy nodes. HAProxy instances spreading gRPC clients over the database nodes. */
  haproxy?: number | string;
  /** Fail domains. How many independent failure units the storage layer actually gets — what the erasure mode is checked against. */
  readonly fail_domains?: number | string;
}
