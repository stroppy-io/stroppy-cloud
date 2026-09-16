package provision

import (
	"fmt"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	ec2 "github.com/upbound/provider-aws/v2/apis/cluster/ec2/v1beta1"

	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// AWSSettings is the baked provider.aws.settings@1 value.
type AWSSettings struct {
	Region           string `json:"region"`
	AvailabilityZone string `json:"availability_zone"`
	Network          struct {
		Kind            string `json:"kind"`
		CIDR            string `json:"cidr"`
		VPCID           string `json:"vpc_id"`
		SubnetID        string `json:"subnet_id"`
		SecurityGroupID string `json:"security_group_id"`
	} `json:"network"`
	InstanceFamily string `json:"instance_family"`
	AMIFamily      string `json:"ami_family"`
	PublicIPs      bool   `json:"public_ips"`
	Spot           bool   `json:"spot"`
}

// AWS provisions through provider-upjet-aws.
type AWS struct{}

// Kind is aws.
func (AWS) Kind() spec.ProviderKind { return spec.ProviderAWS }

// Scheme teaches a k8s client the provider's types.
func (AWS) Scheme() k8slib.ClientOption { return k8slib.WithScheme(ec2.SchemeBuilder.AddToScheme) }

// Record declares one of every kind (see Provider).
func (a AWS) Record(ctx pipeline.Context, k8s *k8slib.Client) {
	k8slib.Resource(ctx, k8s, "record-vpc", &ec2.VPC{}, k8slib.WithReady(awsVPCReady))
	k8slib.Resource(ctx, k8s, "record-subnet", &ec2.Subnet{}, k8slib.WithReady(awsSubnetReady))
	k8slib.Resource(ctx, k8s, "record-igw", &ec2.InternetGateway{}, k8slib.WithReady(awsIGWReady))
	k8slib.Resource(ctx, k8s, "record-rt", &ec2.RouteTable{}, k8slib.WithReady(awsRouteTableReady))
	k8slib.Resource(ctx, k8s, "record-route", &ec2.Route{}, k8slib.WithReady(awsRouteReady))
	k8slib.Resource(ctx, k8s, "record-rta", &ec2.RouteTableAssociation{}, k8slib.WithReady(awsRTAReady))
	k8slib.Resource(ctx, k8s, "record-sg", &ec2.SecurityGroup{}, k8slib.WithReady(awsSGReady))
	k8slib.Resource(ctx, k8s, "record-sgr", &ec2.SecurityGroupRule{}, k8slib.WithReady(awsSGRuleReady))
	k8slib.Resource(ctx, k8s, "record-vm", &ec2.Instance{}, k8slib.WithReady(awsInstanceReady))
}

// Provision declares vpc → subnet, igw → route table → route + association,
// sg → rules, instances.
//
//nolint:funlen // one declaration per resource kind, read top to bottom
func (a AWS) Provision(ctx pipeline.Context, k8s *k8slib.Client, run spec.Run, agents map[string]pipeline.AgentHandle) (Infra, error) {
	st, err := decodeSettings[AWSSettings](run.Provider.Settings)
	if err != nil {
		return Infra{}, err
	}
	if st.Region == "" {
		return Infra{}, fmt.Errorf("provision/aws: region is required")
	}
	names := NewNames(run.Tenant, run.RunID)
	pc := run.Provider.ProviderConfigName
	region := ptr(st.Region)
	public := run.Network.AllowPublicIPs
	cidr := intraCIDR(run)
	tags := labels(run, nil)

	vpcRes := k8slib.Resource(ctx, k8s, names.Network(), &ec2.VPC{
		Spec: ec2.VPCSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: ec2.VPCParameters_2{
				Region:             region,
				CidrBlock:          ptr(cidr),
				EnableDNSHostnames: ptr(true),
				EnableDNSSupport:   ptr(true),
				Tags:               withName(tags, names.Network()),
			},
		},
	}, k8slib.WithReady(awsVPCReady))

	sub := k8slib.Resource(ctx, k8s, names.Subnet(), &ec2.Subnet{
		Spec: ec2.SubnetSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: ec2.SubnetParameters_2{
				Region:              region,
				CidrBlock:           ptr(cidr),
				AvailabilityZone:    optional(st.AvailabilityZone),
				MapPublicIPOnLaunch: ptr(public),
				VPCIDRef:            &xpv1.Reference{Name: names.Network()},
				Tags:                withName(tags, names.Subnet()),
			},
		},
	}, k8slib.WithReady(awsSubnetReady), k8slib.WithResourceOption[ec2.Subnet](pipeline.Parent(vpcRes)))

	igw := k8slib.Resource(ctx, k8s, names.Gateway(), &ec2.InternetGateway{
		Spec: ec2.InternetGatewaySpec{
			ResourceSpec: providerRef(pc),
			ForProvider: ec2.InternetGatewayParameters_2{
				Region:   region,
				VPCIDRef: &xpv1.Reference{Name: names.Network()},
				Tags:     withName(tags, names.Gateway()),
			},
		},
	}, k8slib.WithReady(awsIGWReady), k8slib.WithResourceOption[ec2.InternetGateway](pipeline.Parent(vpcRes)))

	rt := k8slib.Resource(ctx, k8s, names.RouteTable(), &ec2.RouteTable{
		Spec: ec2.RouteTableSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: ec2.RouteTableParameters_2{
				Region:   region,
				VPCIDRef: &xpv1.Reference{Name: names.Network()},
				Tags:     withName(tags, names.RouteTable()),
			},
		},
	}, k8slib.WithReady(awsRouteTableReady), k8slib.WithResourceOption[ec2.RouteTable](pipeline.Parent(igw)))

	k8slib.Resource(ctx, k8s, names.RouteTable()+"-default", &ec2.Route{
		Spec: ec2.RouteSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: ec2.RouteParameters_2{
				Region:               region,
				RouteTableIDRef:      &xpv1.Reference{Name: names.RouteTable()},
				DestinationCidrBlock: ptr("0.0.0.0/0"),
				GatewayIDRef:         &xpv1.Reference{Name: names.Gateway()},
			},
		},
	}, k8slib.WithReady(awsRouteReady), k8slib.WithResourceOption[ec2.Route](pipeline.Parent(rt)))

	k8slib.Resource(ctx, k8s, names.RouteTable()+"-assoc", &ec2.RouteTableAssociation{
		Spec: ec2.RouteTableAssociationSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: ec2.RouteTableAssociationParameters{
				Region:          region,
				RouteTableIDRef: &xpv1.Reference{Name: names.RouteTable()},
				SubnetIDRef:     &xpv1.Reference{Name: names.Subnet()},
			},
		},
	}, k8slib.WithReady(awsRTAReady), k8slib.WithResourceOption[ec2.RouteTableAssociation](pipeline.Parent(rt)))

	sg := k8slib.Resource(ctx, k8s, names.SecurityGroup(), &ec2.SecurityGroup{
		Spec: ec2.SecurityGroupSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: ec2.SecurityGroupParameters_2{
				Region:      region,
				Name:        ptr(names.SecurityGroup()),
				Description: ptr("stroppy run " + run.RunID),
				VPCIDRef:    &xpv1.Reference{Name: names.Network()},
				Tags:        withName(tags, names.SecurityGroup()),
			},
		},
	}, k8slib.WithReady(awsSGReady), k8slib.WithResourceOption[ec2.SecurityGroup](pipeline.Parent(sub)))

	for i, rule := range awsRules(run, cidr) {
		rule.Region = region
		rule.SecurityGroupIDRef = &xpv1.Reference{Name: names.SecurityGroup()}
		k8slib.Resource(ctx, k8s, names.Rule(*rule.Type, i), &ec2.SecurityGroupRule{
			Spec: ec2.SecurityGroupRuleSpec{ResourceSpec: providerRef(pc), ForProvider: rule},
		}, k8slib.WithReady(awsSGRuleReady), k8slib.WithResourceOption[ec2.SecurityGroupRule](pipeline.Parent(sg)))
	}

	infra := Infra{Root: vpcRes, Machines: map[string]*Machine{}}
	for _, m := range run.Machines {
		agent, ok := agents[m.Name]
		if !ok {
			return Infra{}, fmt.Errorf("provision/aws: no agent declared for machine %q", m.Name)
		}
		// Secondary disks ride along as EBS block devices: EC2 creates and
		// deletes them with the instance, no separate record needed.
		var ebs []ec2.InstanceEBSBlockDeviceParameters
		for i, d := range m.Disks {
			ebs = append(ebs, ec2.InstanceEBSBlockDeviceParameters{
				DeviceName:          ptr(awsDeviceName(i)),
				VolumeSize:          ptr(float64(d.GB)),
				VolumeType:          ptr(d.Type),
				DeleteOnTermination: ptr(true),
				Tags:                withName(tags, names.Disk(m.Name, d.Name)),
			})
		}
		bootGB, bootType := awsBootDiskGB, "gp3"
		if m.BootDisk != nil {
			bootGB, bootType = m.BootDisk.GB, m.BootDisk.Type
		}
		vmPublic, spot := public, st.Spot
		if m.PublicIP != nil {
			vmPublic = *m.PublicIP
		}
		if m.Preemptible != nil {
			spot = *m.Preemptible
		}
		vmName := names.Machine(m.Name)
		params := ec2.InstanceParameters{
			Region:                   region,
			AMI:                      ptr(m.Image),
			InstanceType:             ptr(m.InstanceType),
			AvailabilityZone:         optional(st.AvailabilityZone),
			AssociatePublicIPAddress: ptr(vmPublic),
			SubnetIDRef:              &xpv1.Reference{Name: names.Subnet()},
			VPCSecurityGroupIDRefs:   []xpv1.Reference{{Name: names.SecurityGroup()}},
			UserData:                 ptr(agent.CloudInit()),
			UserDataReplaceOnChange:  ptr(false),
			RootBlockDevice: []ec2.RootBlockDeviceParameters{{
				VolumeSize:          ptr(float64(bootGB)),
				VolumeType:          ptr(bootType),
				DeleteOnTermination: ptr(true),
			}},
			EBSBlockDevice: ebs,
			MetadataOptions: []ec2.MetadataOptionsParameters{{
				HTTPEndpoint: ptr("enabled"), HTTPTokens: ptr("required"),
			}},
			Tags: withName(labels(run, m.Labels), vmName),
		}
		if spot {
			params.InstanceMarketOptions = []ec2.InstanceMarketOptionsParameters{{MarketType: ptr("spot")}}
		}
		vm := k8slib.Resource(ctx, k8s, vmName, &ec2.Instance{
			Spec: ec2.InstanceSpec{ResourceSpec: providerRef(pc), ForProvider: params},
		},
			k8slib.WithReady(awsInstanceReady),
			k8slib.WithTimeout[ec2.Instance](machineTimeout),
			k8slib.WithResourceOption[ec2.Instance](pipeline.Parent(sg), pipeline.Children(agent)),
		)
		mach := &Machine{Spec: m, Agent: agent, VM: vm}
		mach.wait = func(ctx pipeline.Context) (MachineInfo, error) {
			live, err := vm.TryReady(ctx)
			if err != nil {
				return MachineInfo{}, err
			}
			return awsInfo(live), nil
		}
		infra.Machines[m.Name] = mach
		infra.Order = append(infra.Order, m.Name)
	}
	return infra, nil
}

// awsBootDiskGB is the root volume of every instance.
const awsBootDiskGB = 40

// machineTimeout bounds waiting for one VM to run: cloud capacity errors
// surface here instead of hanging the run.
const machineTimeout = 30 * time.Minute

// awsRules opens the run network to itself, allows all egress, and adds
// the spec's ingress.
func awsRules(run spec.Run, cidr string) []ec2.SecurityGroupRuleParameters_2 {
	rules := []ec2.SecurityGroupRuleParameters_2{
		{Type: ptr("ingress"), Protocol: ptr("-1"), FromPort: ptr(0.0), ToPort: ptr(0.0), CidrBlocks: strPtrs(cidr), Description: ptr("intra-run")},
		{Type: ptr("egress"), Protocol: ptr("-1"), FromPort: ptr(0.0), ToPort: ptr(0.0), CidrBlocks: strPtrs("0.0.0.0/0"), Description: ptr("all egress")},
	}
	for _, in := range run.Network.Ingress {
		proto := "tcp"
		if in.Proto == "udp" {
			proto = "udp"
		}
		rules = append(rules, ec2.SecurityGroupRuleParameters_2{
			Type: ptr("ingress"), Protocol: ptr(proto),
			FromPort: ptr(float64(in.Port)), ToPort: ptr(float64(in.Port)),
			CidrBlocks: strPtrs(in.CIDR), Description: ptr(fmt.Sprintf("ingress %d", in.Port)),
		})
	}
	return rules
}

// awsDeviceName maps the i-th extra disk to /dev/sdf, /dev/sdg, …
func awsDeviceName(i int) string {
	const letters = "fghijklmnop"
	if i < 0 || i >= len(letters) {
		return fmt.Sprintf("/dev/sdz%d", i)
	}
	return "/dev/sd" + string(letters[i])
}

func awsInfo(live *ec2.Instance) MachineInfo {
	info := MachineInfo{}
	if live == nil {
		return info
	}
	if live.Status.AtProvider.ID != nil {
		info.ID = *live.Status.AtProvider.ID
	}
	if live.Status.AtProvider.PrivateIP != nil {
		info.PrivateIP = *live.Status.AtProvider.PrivateIP
	}
	if live.Status.AtProvider.PublicIP != nil {
		info.PublicIP = *live.Status.AtProvider.PublicIP
	}
	return info
}

func withName(tags map[string]*string, name string) map[string]*string {
	out := make(map[string]*string, len(tags)+1)
	for k, v := range tags {
		out[k] = v
	}
	out["Name"] = ptr(name)
	return out
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return ptr(s)
}

func awsVPCReady(live *ec2.VPC) bool { return xpReady(live.Status.GetCondition(xpv1.TypeReady)) }

func awsSubnetReady(live *ec2.Subnet) bool { return xpReady(live.Status.GetCondition(xpv1.TypeReady)) }

func awsIGWReady(live *ec2.InternetGateway) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func awsRouteTableReady(live *ec2.RouteTable) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func awsRouteReady(live *ec2.Route) bool { return xpReady(live.Status.GetCondition(xpv1.TypeReady)) }

func awsRTAReady(live *ec2.RouteTableAssociation) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func awsSGReady(live *ec2.SecurityGroup) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func awsSGRuleReady(live *ec2.SecurityGroupRule) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

// awsInstanceReady: Ready condition AND state "running".
func awsInstanceReady(live *ec2.Instance) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady)) &&
		live.Status.AtProvider.InstanceState != nil && *live.Status.AtProvider.InstanceState == "running"
}
