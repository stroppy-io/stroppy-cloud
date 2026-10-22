// GENERATED from schemapb schema db.ydb.params@1 — do not edit.
// Self-hosted YDB topology: storage/database nodes, erasure mode, pdisks, database path.

/** root */
export interface DbYdbParams1 {
  /** Tuning. auto = settings sized from the machine, role, topology and workload (every value the run used is in its configs); stock = the engine's own defaults. A key set in the config wins over auto. */
  tuning?: "auto" | "stock";
  /** YDB version. ydbd server series; binaries are fetched through the stroppy gateway. */
  version: "26.3" | "26.2" | "26.1" | "25.4";
  /** Erasure mode. config.erasure: none = no redundancy (dev only); block-4-2 = 4 data + 2 parity over >=8 fail domains; mirror-3-dc = 3 realms x 3 domains, >=9 nodes. */
  fault_tolerance: "none" | "block-4-2" | "mirror-3-dc";
  /** Fail domain level. Removed: every storage node is one fail domain. Ignored when present. */
  failure_domain?: string;
  /** Storage nodes. Nodes running the BlobStorage layer; together with pdisks_per_node they supply the fail domains the erasure mode demands. */
  storage_nodes?: number | string;
  /** Database nodes. Nodes running the query/tablet layer of the database; these are what the workload connects to. */
  database_nodes?: number | string;
  /** PDisks per storage node. Drive entries per host_config, each a file of at least 80 GB on the node's data disk (the disk needs 80 GB per pdisk + 20 GB). */
  pdisks_per_node?: number | string;
  /** PDisk device type. Drive type of every host_config entry, the storage pool's pdisk filter and pool kind (ssd / nvme / rot) the database is created in. */
  disk_type: "SSD" | "NVME" | "HDD";
  /** Storage groups. Blob storage groups created in the database's storage pool; more groups spread tablets and their logs wider. Empty = sized by the tuning (auto: 4, mirror-3-dc from the database-node cores; stock: 1). */
  storage_groups?: number | string | null;
  /** Auto-size pdisks. Size each pdisk file from the data disk: 80% of (disk - 10 GB) split over the node's pdisks, at least 80 GB. Off = 80 GB each. */
  auto_size_pdisks?: boolean;
  /** Database path. Full path of the benchmark database inside the cluster; /Root itself is reserved for the cluster root. */
  database_path?: string;
  /** TLS (grpcs). Removed until the stand issues certificates: the endpoint is plain grpc://. Ignored when present. */
  grpcs?: boolean;
  /** HAProxy nodes. HAProxy instances spreading gRPC clients over the database nodes. */
  haproxy?: number | string;
  /** Fail domains. How many independent failure units the storage layer gets: one per storage node — what the erasure mode is checked against. */
  readonly fail_domains?: number | string;
}
