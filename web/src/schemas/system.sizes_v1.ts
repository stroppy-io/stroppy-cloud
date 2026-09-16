// GENERATED from schemapb schema system.sizes@1 — do not edit.
// Platform size table: provider → role family → XS..XL → concrete machine.

/** object XS */
export interface SystemSizes1YandexValueXS {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** object S */
export interface SystemSizes1YandexValueS {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** object M */
export interface SystemSizes1YandexValueM {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** object L */
export interface SystemSizes1YandexValueL {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** object XL */
export interface SystemSizes1YandexValueXL {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** map value yandex */
export interface SystemSizes1YandexValue {
  /** XS. What size XS means for this role family. */
  XS: SystemSizes1YandexValueXS;
  /** S. What size S means for this role family. */
  S: SystemSizes1YandexValueS;
  /** M. What size M means for this role family. */
  M: SystemSizes1YandexValueM;
  /** L. What size L means for this role family. */
  L: SystemSizes1YandexValueL;
  /** XL. What size XL means for this role family. */
  XL: SystemSizes1YandexValueXL;
}

/** object XS */
export interface SystemSizes1AwsValueXS {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** object S */
export interface SystemSizes1AwsValueS {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** object M */
export interface SystemSizes1AwsValueM {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** object L */
export interface SystemSizes1AwsValueL {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** object XL */
export interface SystemSizes1AwsValueXL {
  /** vCPU. Cores the machine gets. */
  cpu: number | string;
  /** Memory. RAM the machine gets. [GB] */
  memory_gb: number | string;
  /** Instance type. Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge). */
  instance_type: string;
  /** Default disk. Data disk size when the test does not override it. [GB] */
  default_disk_gb: number | string;
  /** Disk type. Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws). */
  disk_type: string;
}

/** map value aws */
export interface SystemSizes1AwsValue {
  /** XS. What size XS means for this role family. */
  XS: SystemSizes1AwsValueXS;
  /** S. What size S means for this role family. */
  S: SystemSizes1AwsValueS;
  /** M. What size M means for this role family. */
  M: SystemSizes1AwsValueM;
  /** L. What size L means for this role family. */
  L: SystemSizes1AwsValueL;
  /** XL. What size XL means for this role family. */
  XL: SystemSizes1AwsValueXL;
}

/** root */
export interface SystemSizes1 {
  /** yandex. Keys are role families (db, proxy, runner, coordinator); each maps XS..XL to a machine. */
  yandex?: Record<string, SystemSizes1YandexValue>;
  /** aws. Keys are role families (db, proxy, runner, coordinator); each maps XS..XL to a machine. */
  aws?: Record<string, SystemSizes1AwsValue>;
}
