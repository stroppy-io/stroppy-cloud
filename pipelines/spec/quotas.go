package spec

// QuotaKind is a provider quota the quotas pipeline reports and its unit.
type QuotaKind struct {
	Name string
	Unit string
}

// QuotaKinds is every quota the providers report (the server contract's
// QuotaName/QuotaUnit are these names and units): Yandex Cloud Quota
// Manager ids, then AWS Service Quotas under our names.
//
// doc: https://yandex.cloud/en/docs/compute/concepts/limits#compute-quotas
// doc: https://docs.aws.amazon.com/general/latest/gr/ec2-service.html#limits_ec2
var QuotaKinds = []QuotaKind{
	{"compute.instances.count", "instances"},
	{"compute.instanceCores.count", "cores"},
	{"compute.instanceMemory.size", "bytes"},
	{"compute.ssdDisks.size", "bytes"},
	{"compute.hddDisks.size", "bytes"},
	{"compute.ssdNonReplicatedDisks.size", "bytes"},
	{"compute.ssdIOM3Disks.size", "bytes"},
	{"compute.instanceHighFreqCores.count", "cores"},
	{"compute.disks.count", "disks"},
	{"vpc.networks.count", "networks"},
	{"vpc.subnets.count", "subnets"},
	{"vpc.securityGroups.count", "security-groups"},
	{"vpc.externalStaticAddresses.count", "addresses"},
	{"ec2.standard.vcpus", "vcpus"},
	{"ec2.spot.standard.vcpus", "vcpus"},
	{"ebs.gp3.storage", "tib"},
	{"ebs.gp2.storage", "tib"},
	{"ebs.io2.storage", "tib"},
	{"vpc.vpcs", "vpcs"},
	{"vpc.security_groups", "groups"},
	{"vpc.internet_gateways", "gateways"},
	{"ec2.elastic_ips", "addresses"},
}
