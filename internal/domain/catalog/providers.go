package catalog

// providers is the v0 provider table: what the settings schemas offer,
// plus the size table (system.sizes@1) per role family.
//
// doc: https://yandex.cloud/en/docs/compute/concepts/vm-platforms
// doc: https://yandex.cloud/en/docs/compute/concepts/disk#disks-types
// doc: https://yandex.cloud/en/docs/compute/concepts/limits — disk and VM IOPS/bandwidth
// doc: https://aws.amazon.com/ec2/instance-types/
// doc: https://docs.aws.amazon.com/ebs/latest/userguide/ebs-volume-types.html
//
//nolint:gosec // schema ids and instance types, not credentials
func providers() []Provider {
	return []Provider{
		{
			Kind: Yandex, Title: "Yandex Cloud",
			SettingsSchema: "provider.yandex.settings@1", CredentialsSchema: "provider.yandex.credentials@1",
			Locations: []Named{
				{ID: "ru-central1-a", Title: "ru-central1-a"},
				{ID: "ru-central1-b", Title: "ru-central1-b"},
				{ID: "ru-central1-d", Title: "ru-central1-d (recommended)"},
				{ID: "ru-central1-e", Title: "ru-central1-e"},
			},
			Platforms: []Named{
				{ID: "standard-v3", Title: "Intel Ice Lake (standard-v3)"},
				{ID: "standard-v2", Title: "Intel Cascade Lake (standard-v2)"},
				{ID: "standard-v1", Title: "Intel Broadwell (standard-v1)"},
				{ID: "highfreq-v3", Title: "Intel Ice Lake compute-optimized (highfreq-v3)"},
			},
			DiskTypes: []DiskType{
				{ID: "network-ssd", Title: "Network SSD", MinGB: 4, StepGB: 4, Perf: DiskPerf{UnitGB: 32, ReadIOPSPerUnit: 1000, WriteIOPSPerUnit: 1000, ReadMBpsPerUnit: 15, WriteMBpsPerUnit: 15, MaxReadIOPS: 20000, MaxWriteIOPS: 40000, MaxReadMBps: 450, MaxWriteMBps: 450, VMIOPSPerCPU: 3500, VMMBpsPerCPU: 45}},
				{ID: "network-ssd-nonreplicated", Title: "Non-replicated SSD", MinGB: 93, StepGB: 93, Perf: DiskPerf{UnitGB: 93, ReadIOPSPerUnit: 28000, WriteIOPSPerUnit: 5600, ReadMBpsPerUnit: 110, WriteMBpsPerUnit: 82, MaxReadIOPS: 75000, MaxWriteIOPS: 75000, MaxReadMBps: 1024, MaxWriteMBps: 1024, VMIOPSPerCPU: 10000, VMMBpsPerCPU: 100}},
				{ID: "network-ssd-io-m3", Title: "Ultra high-speed SSD (io-m3)", MinGB: 93, StepGB: 93, Perf: DiskPerf{UnitGB: 93, ReadIOPSPerUnit: 28000, WriteIOPSPerUnit: 5600, ReadMBpsPerUnit: 110, WriteMBpsPerUnit: 82, MaxReadIOPS: 75000, MaxWriteIOPS: 40000, MaxReadMBps: 1024, MaxWriteMBps: 1024, VMIOPSPerCPU: 10000, VMMBpsPerCPU: 100}},
				{ID: "network-hdd", Title: "Network HDD", MinGB: 4, StepGB: 4, Perf: DiskPerf{UnitGB: 256, ReadIOPSPerUnit: 300, WriteIOPSPerUnit: 300, ReadMBpsPerUnit: 30, WriteMBpsPerUnit: 30, MaxReadIOPS: 2000, MaxWriteIOPS: 11000, MaxReadMBps: 240, MaxWriteMBps: 240, VMIOPSPerCPU: 3500, VMMBpsPerCPU: 45}},
			},
			Sizes: map[string]map[string]SizeSpec{
				"db": {
					"XS": {CPU: 2, MemoryGB: 8, InstanceType: "standard-v3", DefaultDiskGB: 50, DiskType: "network-ssd"},
					"S":  {CPU: 4, MemoryGB: 16, InstanceType: "standard-v3", DefaultDiskGB: 100, DiskType: "network-ssd"},
					"M":  {CPU: 8, MemoryGB: 32, InstanceType: "standard-v3", DefaultDiskGB: 200, DiskType: "network-ssd"},
					"L":  {CPU: 16, MemoryGB: 64, InstanceType: "standard-v3", DefaultDiskGB: 400, DiskType: "network-ssd-io-m3"},
					"XL": {CPU: 32, MemoryGB: 128, InstanceType: "standard-v3", DefaultDiskGB: 800, DiskType: "network-ssd-io-m3"},
				},
				"proxy": {
					"XS": {CPU: 2, MemoryGB: 4, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
					"S":  {CPU: 2, MemoryGB: 8, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
					"M":  {CPU: 4, MemoryGB: 8, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
					"L":  {CPU: 8, MemoryGB: 16, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
					"XL": {CPU: 16, MemoryGB: 32, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
				},
				"runner": {
					"XS": {CPU: 2, MemoryGB: 4, InstanceType: "standard-v3", DefaultDiskGB: 30, DiskType: "network-ssd"},
					"S":  {CPU: 4, MemoryGB: 8, InstanceType: "standard-v3", DefaultDiskGB: 30, DiskType: "network-ssd"},
					"M":  {CPU: 8, MemoryGB: 16, InstanceType: "standard-v3", DefaultDiskGB: 30, DiskType: "network-ssd"},
					"L":  {CPU: 16, MemoryGB: 32, InstanceType: "standard-v3", DefaultDiskGB: 30, DiskType: "network-ssd"},
					"XL": {CPU: 32, MemoryGB: 64, InstanceType: "standard-v3", DefaultDiskGB: 30, DiskType: "network-ssd"},
				},
				"coordinator": {
					"XS": {CPU: 2, MemoryGB: 4, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
					"S":  {CPU: 2, MemoryGB: 4, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
					"M":  {CPU: 2, MemoryGB: 8, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
					"L":  {CPU: 4, MemoryGB: 8, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
					"XL": {CPU: 4, MemoryGB: 16, InstanceType: "standard-v3", DefaultDiskGB: 20, DiskType: "network-ssd"},
				},
			},
			Images: []Image{
				{ID: "ubuntu-2404-lts", OS: "Ubuntu 24.04 LTS"},
				{ID: "ubuntu-2204-lts", OS: "Ubuntu 22.04 LTS"},
				{ID: "ubuntu-2004-lts", OS: "Ubuntu 20.04 LTS"},
			},
		},
		{
			Kind: AWS, Title: "Amazon Web Services",
			SettingsSchema: "provider.aws.settings@1", CredentialsSchema: "provider.aws.credentials@1",
			Locations: []Named{
				{ID: "eu-central-1", Title: "Europe (Frankfurt)"},
				{ID: "eu-west-1", Title: "Europe (Ireland)"},
				{ID: "eu-north-1", Title: "Europe (Stockholm)"},
				{ID: "us-east-1", Title: "US East (N. Virginia)"},
				{ID: "us-east-2", Title: "US East (Ohio)"},
				{ID: "us-west-2", Title: "US West (Oregon)"},
				{ID: "ap-southeast-1", Title: "Asia Pacific (Singapore)"},
				{ID: "ap-northeast-1", Title: "Asia Pacific (Tokyo)"},
			},
			Platforms: []Named{
				{ID: "m7i", Title: "m7i — general purpose, Sapphire Rapids"},
				{ID: "m7a", Title: "m7a — general purpose, AMD Genoa"},
				{ID: "c7i", Title: "c7i — compute optimized, Sapphire Rapids"},
			},
			DiskTypes: []DiskType{
				{ID: "gp3", Title: "General Purpose SSD (gp3)", MinGB: 1, StepGB: 1, Perf: DiskPerf{ReadIOPSPerUnit: 3000, WriteIOPSPerUnit: 3000, ReadMBpsPerUnit: 125, WriteMBpsPerUnit: 125}},
				{ID: "io2", Title: "Provisioned IOPS SSD (io2)", MinGB: 4, StepGB: 1},
				{ID: "st1", Title: "Throughput Optimized HDD (st1)", MinGB: 125, StepGB: 1},
			},
			Sizes: map[string]map[string]SizeSpec{
				"db": {
					"XS": {CPU: 2, MemoryGB: 8, InstanceType: "m7i.large", DefaultDiskGB: 50, DiskType: "gp3"},
					"S":  {CPU: 4, MemoryGB: 16, InstanceType: "m7i.xlarge", DefaultDiskGB: 100, DiskType: "gp3"},
					"M":  {CPU: 8, MemoryGB: 32, InstanceType: "m7i.2xlarge", DefaultDiskGB: 200, DiskType: "gp3"},
					"L":  {CPU: 16, MemoryGB: 64, InstanceType: "m7i.4xlarge", DefaultDiskGB: 400, DiskType: "io2"},
					"XL": {CPU: 32, MemoryGB: 128, InstanceType: "m7i.8xlarge", DefaultDiskGB: 800, DiskType: "io2"},
				},
				"proxy": {
					"XS": {CPU: 2, MemoryGB: 4, InstanceType: "c7i.large", DefaultDiskGB: 20, DiskType: "gp3"},
					"S":  {CPU: 2, MemoryGB: 4, InstanceType: "c7i.large", DefaultDiskGB: 20, DiskType: "gp3"},
					"M":  {CPU: 4, MemoryGB: 8, InstanceType: "c7i.xlarge", DefaultDiskGB: 20, DiskType: "gp3"},
					"L":  {CPU: 8, MemoryGB: 16, InstanceType: "c7i.2xlarge", DefaultDiskGB: 20, DiskType: "gp3"},
					"XL": {CPU: 16, MemoryGB: 32, InstanceType: "c7i.4xlarge", DefaultDiskGB: 20, DiskType: "gp3"},
				},
				"runner": {
					"XS": {CPU: 2, MemoryGB: 4, InstanceType: "c7i.large", DefaultDiskGB: 30, DiskType: "gp3"},
					"S":  {CPU: 4, MemoryGB: 8, InstanceType: "c7i.xlarge", DefaultDiskGB: 30, DiskType: "gp3"},
					"M":  {CPU: 8, MemoryGB: 16, InstanceType: "c7i.2xlarge", DefaultDiskGB: 30, DiskType: "gp3"},
					"L":  {CPU: 16, MemoryGB: 32, InstanceType: "c7i.4xlarge", DefaultDiskGB: 30, DiskType: "gp3"},
					"XL": {CPU: 32, MemoryGB: 64, InstanceType: "c7i.8xlarge", DefaultDiskGB: 30, DiskType: "gp3"},
				},
				"coordinator": {
					"XS": {CPU: 2, MemoryGB: 4, InstanceType: "c7i.large", DefaultDiskGB: 20, DiskType: "gp3"},
					"S":  {CPU: 2, MemoryGB: 4, InstanceType: "c7i.large", DefaultDiskGB: 20, DiskType: "gp3"},
					"M":  {CPU: 2, MemoryGB: 8, InstanceType: "m7i.large", DefaultDiskGB: 20, DiskType: "gp3"},
					"L":  {CPU: 4, MemoryGB: 16, InstanceType: "m7i.xlarge", DefaultDiskGB: 20, DiskType: "gp3"},
					"XL": {CPU: 4, MemoryGB: 16, InstanceType: "m7i.xlarge", DefaultDiskGB: 20, DiskType: "gp3"},
				},
			},
			Images: []Image{
				{ID: "ubuntu-24.04", OS: "Ubuntu 24.04 LTS (noble)"},
				{ID: "ubuntu-22.04", OS: "Ubuntu 22.04 LTS (jammy)"},
			},
		},
	}
}
