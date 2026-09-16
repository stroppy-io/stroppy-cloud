// GENERATED from schemapb schema provider.aws.settings@1 — do not edit.
// AWS placement settings of a tenant provider profile (no secrets).

/** variant create of network */
export interface ProviderAwsSettings1NetworkCreate {
  kind: "create";
  /** VPC CIDR. IPv4 range of the VPC created for the run. */
  cidr?: string;
}

/** variant existing of network */
export interface ProviderAwsSettings1NetworkExisting {
  kind: "existing";
  /** VPC id. Existing VPC the machines join. */
  vpc_id: string;
  /** Subnet id. Existing subnet in the chosen availability zone. */
  subnet_id: string;
  /** Security group id. Existing security group applied to every machine. */
  security_group_id?: string;
}

/** root */
export interface ProviderAwsSettings1 {
  /** Region. AWS commercial region the run is created in; opt-in regions must be enabled on the account. */
  region: "us-east-1" | "us-east-2" | "us-west-1" | "us-west-2" | "af-south-1" | "ap-east-1" | "ap-east-2" | "ap-south-1" | "ap-south-2" | "ap-northeast-1" | "ap-northeast-2" | "ap-northeast-3" | "ap-southeast-1" | "ap-southeast-2" | "ap-southeast-3" | "ap-southeast-4" | "ap-southeast-5" | "ap-southeast-6" | "ap-southeast-7" | "ca-central-1" | "ca-west-1" | "eu-central-1" | "eu-central-2" | "eu-west-1" | "eu-west-2" | "eu-west-3" | "eu-north-1" | "eu-south-1" | "eu-south-2" | "il-central-1" | "mx-central-1" | "me-south-1" | "me-central-1" | "sa-east-1";
  /** Availability zone. Availability zone inside the region; empty lets AWS pick one. */
  availability_zone?: string;
  /** Network. Create a throwaway VPC for every run, or place runs into an existing one. */
  network: ProviderAwsSettings1NetworkCreate | ProviderAwsSettings1NetworkExisting;
  /** Instance family. EC2 family the size table resolves instance types in. */
  instance_family?: "m6i" | "m7i" | "m6a" | "m7a" | "c6i" | "c7i" | "r6i" | "r7i";
  /** AMI family. Boot image family; the concrete AMI id is resolved per region at launch. */
  ami_family?: "ubuntu-24.04" | "ubuntu-22.04" | "al2023";
  /** Public IPs. Give every machine a public IPv4 address. */
  public_ips?: boolean;
  /** Spot instances. Use Spot capacity: much cheaper, but interruptible with a two-minute notice — do not use for a measurement that must complete. */
  spot?: boolean;
}
