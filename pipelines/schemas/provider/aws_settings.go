package provider

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// awsRegions is the AWS commercial partition, as published by the regions
// reference. GovCloud (us-gov-*) and China (cn-*) are separate partitions and
// are deliberately absent.
// doc: https://docs.aws.amazon.com/global-infrastructure/latest/regions/aws-regions.html
var awsRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"af-south-1",
	"ap-east-1", "ap-east-2", "ap-south-1", "ap-south-2",
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
	"ap-southeast-1", "ap-southeast-2", "ap-southeast-3", "ap-southeast-4",
	"ap-southeast-5", "ap-southeast-6", "ap-southeast-7",
	"ca-central-1", "ca-west-1",
	"eu-central-1", "eu-central-2",
	"eu-west-1", "eu-west-2", "eu-west-3", "eu-north-1",
	"eu-south-1", "eu-south-2",
	"il-central-1", "mx-central-1",
	"me-south-1", "me-central-1",
	"sa-east-1",
}

// AwsSettings is provider.aws.settings@1 — everything about an AWS target that
// is safe to store and show. Credentials live in provider.aws.credentials@1.
func AwsSettings() *schemapb.Schema {
	region := schemapb.Choice("region").Title("Region").Group("Placement").
		Desc("AWS commercial region the run is created in; opt-in regions must be enabled on the account.")
	for _, r := range awsRegions {
		region = region.Opt(schemapb.StrV(r), r)
	}

	return schemapb.NewSchema(ids.Provider("aws", "settings", 1)).
		Descr("AWS placement settings of a tenant provider profile (no secrets).").
		Strict().Coerce().
		Fields(
			region.Default(schemapb.StrV("eu-central-1")).Required(),

			// doc: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html
			// — an AZ id is the region code plus a single lowercase letter.
			schemapb.Str("availability_zone").Title("Availability zone").Group("Placement").
				Desc("Availability zone inside the region; empty lets AWS pick one.").
				Pattern(`^[a-z]{2}(-[a-z]+)+-\d[a-z]$`).MaxLen(24).
				Examples(schemapb.StrV("eu-central-1a")),

			schemapb.OneOf("network", "kind").Title("Network").Group("Network").
				Desc("Create a throwaway VPC for every run, or place runs into an existing one.").
				Variant("create",
					schemapb.Str("cidr").Title("VPC CIDR").
						Desc("IPv4 range of the VPC created for the run.").
						Pattern(`^(\d{1,3}\.){3}\d{1,3}/\d{1,2}$`).Default("10.130.0.0/16"),
				).
				Variant("existing",
					// doc: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html
					// — resource ids are a type prefix plus 8 or 17 hex chars.
					schemapb.Str("vpc_id").Title("VPC id").
						Desc("Existing VPC the machines join.").
						Pattern(`^vpc-[0-9a-f]{8}([0-9a-f]{9})?$`).Required(),
					schemapb.Str("subnet_id").Title("Subnet id").
						Desc("Existing subnet in the chosen availability zone.").
						Pattern(`^subnet-[0-9a-f]{8}([0-9a-f]{9})?$`).Required(),
					schemapb.Str("security_group_id").Title("Security group id").
						Desc("Existing security group applied to every machine.").
						Pattern(`^sg-[0-9a-f]{8}([0-9a-f]{9})?$`),
				).
				Required(),

			// doc: https://docs.aws.amazon.com/ec2/latest/instancetypes/gp.html ,
			// .../co.html , .../mo.html — current-generation x86 families. The
			// concrete instance type comes from system.sizes@1; this only picks
			// the family the size table is read in.
			schemapb.Choice("instance_family").Title("Instance family").Group("Placement").
				Desc("EC2 family the size table resolves instance types in.").
				Opt(schemapb.StrV("m6i"), "m6i — general purpose, Ice Lake").
				Opt(schemapb.StrV("m7i"), "m7i — general purpose, Sapphire Rapids").
				Opt(schemapb.StrV("m6a"), "m6a — general purpose, AMD Milan").
				Opt(schemapb.StrV("m7a"), "m7a — general purpose, AMD Genoa").
				Opt(schemapb.StrV("c6i"), "c6i — compute optimized, Ice Lake").
				Opt(schemapb.StrV("c7i"), "c7i — compute optimized, Sapphire Rapids").
				Opt(schemapb.StrV("r6i"), "r6i — memory optimized, Ice Lake").
				Opt(schemapb.StrV("r7i"), "r7i — memory optimized, Sapphire Rapids").
				Default(schemapb.StrV("m7i")),

			// doc: https://ubuntu.com/aws/docs/aws-how-to/instances/find-ubuntu-images/
			// (Canonical owner 099720109477, noble AMIs are hvm-ssd-gp3) and
			// https://docs.aws.amazon.com/linux/al2023/ug/ec2.html (SSM parameter
			// /aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64).
			schemapb.Choice("ami_family").Title("AMI family").Group("Placement").
				Desc("Boot image family; the concrete AMI id is resolved per region at launch.").
				Opt(schemapb.StrV("ubuntu-24.04"), "Ubuntu 24.04 LTS (noble)").
				Opt(schemapb.StrV("ubuntu-22.04"), "Ubuntu 22.04 LTS (jammy)").
				Opt(schemapb.StrV("al2023"), "Amazon Linux 2023").
				Default(schemapb.StrV("ubuntu-24.04")),

			schemapb.Bool("public_ips").Title("Public IPs").Group("Network").
				Desc("Give every machine a public IPv4 address.").Default(true),

			// doc: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-spot-instances.html
			schemapb.Bool("spot").Title("Spot instances").Group("Placement").
				Desc("Use Spot capacity: much cheaper, but interruptible with a two-minute notice — "+
					"do not use for a measurement that must complete.").
				Default(false),
		).
		Rules(schemapb.Rule(`[this == null ? root : this].all(s, !("network" in s) || s.network.kind == "create")`, "existing networks are not implemented by the pipeline; use a run-owned network").ID("network-create-only")).
		MustBuild()
}
