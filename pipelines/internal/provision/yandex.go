package provision

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	compute "github.com/yandex-cloud/crossplane-provider-yc/apis/cluster/compute/v1alpha1"
	vpc "github.com/yandex-cloud/crossplane-provider-yc/apis/cluster/vpc/v1alpha1"
	ydb "github.com/yandex-cloud/crossplane-provider-yc/apis/cluster/ydb/v1alpha1"

	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// YandexSettings is the baked provider.yandex.settings@1 value.
type YandexSettings struct {
	CloudID     string `json:"cloud_id"`
	FolderID    string `json:"folder_id"`
	Zone        string `json:"zone"`
	PlatformID  string `json:"platform_id"`
	PublicIPs   bool   `json:"public_ips"`
	ImageFamily string `json:"image_family"`
	Preemptible bool   `json:"preemptible"`
}

// Yandex provisions through crossplane-provider-yc.
type Yandex struct{}

// Kind is yandex.
func (Yandex) Kind() spec.ProviderKind { return spec.ProviderYandex }

// Scheme teaches a k8s client the provider's types.
func (Yandex) Scheme() k8slib.ClientOption {
	return k8slib.WithScheme(compute.SchemeBuilder.AddToScheme, vpc.SchemeBuilder.AddToScheme, ydb.SchemeBuilder.AddToScheme)
}

// Record declares one of every kind (see Provider).
func (y Yandex) Record(ctx pipeline.Context, k8s *k8slib.Client) {
	k8slib.Resource(ctx, k8s, "record-ydb-serverless", &ydb.DatabaseServerless{}, k8slib.WithReady(yandexServerlessReady), failedOn[ydb.DatabaseServerless]())
	k8slib.Resource(ctx, k8s, "record-ydb-dedicated", &ydb.DatabaseDedicated{}, k8slib.WithReady(yandexDedicatedReady), failedOn[ydb.DatabaseDedicated]())
	k8slib.Resource(ctx, k8s, "record-nat", &vpc.Gateway{}, k8slib.WithReady(yandexGatewayReady), failedOn[vpc.Gateway]())
	k8slib.Resource(ctx, k8s, "record-routes", &vpc.RouteTable{}, k8slib.WithReady(yandexRouteTableReady), failedOn[vpc.RouteTable]())
	k8slib.Resource(ctx, k8s, "record-net", &vpc.Network{}, k8slib.WithReady(yandexNetworkReady), failedOn[vpc.Network]())
	k8slib.Resource(ctx, k8s, "record-subnet", &vpc.Subnet{}, k8slib.WithReady(yandexSubnetReady), failedOn[vpc.Subnet]())
	k8slib.Resource(ctx, k8s, "record-sg", &vpc.SecurityGroup{}, k8slib.WithReady(yandexSGReady), failedOn[vpc.SecurityGroup]())
	k8slib.Resource(ctx, k8s, "record-disk", &compute.Disk{}, k8slib.WithReady(yandexDiskReady), failedOn[compute.Disk]())
	k8slib.Resource(ctx, k8s, "record-vm", &compute.Instance{}, k8slib.WithReady(yandexInstanceReady), failedOn[compute.Instance]())
}

// Provision declares the run's part of the profile network: a security
// group, an optional own NAT gateway with its route table, the zonal
// subnets the server allocated, then disks + instances.
//
//nolint:funlen // one declaration per resource kind, read top to bottom
func (y Yandex) Provision(ctx pipeline.Context, k8s *k8slib.Client, run spec.Run, agents map[string]pipeline.AgentHandle, shared Shared) (Infra, error) {
	st, err := decodeSettings[YandexSettings](run.Provider.Settings)
	if err != nil {
		return Infra{}, err
	}
	if st.FolderID == "" {
		return Infra{}, fmt.Errorf("provision/yandex: folder_id is required")
	}
	network, err := spec.YandexNetworkOf(run.Provider.Settings)
	if err != nil {
		return Infra{}, fmt.Errorf("provision/yandex: %w", err)
	}
	names := NewNames(run.Tenant, run.RunID)
	pc := run.Provider.ProviderConfigName
	subnets, err := yandexSubnets(run, names.Subnet())
	if err != nil {
		return Infra{}, err
	}
	public := run.Network.AllowPublicIPs
	netRef, netID := yandexNetworkRef(network, pc)

	// The network belongs to the profile, not the run: the run's tree is
	// security group → (own gateway → routes) → zonal subnets → VMs/disks.
	// Ownership keeps every subnet until its VMs are gone and the group
	// until all subnets are gone; moving the group to a stand preserves the
	// whole run and deleting it never reaches the network.
	sg := k8slib.Resource(ctx, k8s, names.SecurityGroup(), &vpc.SecurityGroup{
		Spec: vpc.SecurityGroupSpec{
			ResourceSpec: providerRef(pc),
			ForProvider: vpc.SecurityGroupParameters{
				FolderID:     ptr(st.FolderID),
				Name:         ptr(names.SecurityGroup()),
				NetworkID:    netID,
				NetworkIDRef: netRef,
				Ingress:      yandexIngress(run, subnets),
				Egress: []vpc.SecurityGroupEgressParameters{{
					Description:  ptr("all egress"),
					Protocol:     ptr("ANY"),
					FromPort:     ptr(0.0),
					ToPort:       ptr(65535.0),
					V4CidrBlocks: strPtrs("0.0.0.0/0"),
				}},
			},
		},
	}, k8slib.WithReady(yandexSGReady), failedOn[vpc.SecurityGroup]())

	byZone := map[string]pipeline.Handle{}
	subnetReady := map[string]readyFn{}
	var subnetWaiters []func(pipeline.Context) error
	var parent pipeline.Handle = sg
	var routeID *string
	if spec.NeedsPrivateEgress(run) {
		switch {
		case shared.RouteTableID != "":
			routeID = ptr(shared.RouteTableID)
		case network.SharedEgress():
			return Infra{}, fmt.Errorf("provision/yandex: the profile network reports no route table id; configure the provider again")
		default:
			// The profile network has no route out: this run brings its own
			// NAT gateway and route table, owned by its security group.
			gateway := k8slib.Resource(ctx, k8s, names.Gateway(), &vpc.Gateway{Spec: vpc.GatewaySpec{ResourceSpec: providerRef(pc), ForProvider: vpc.GatewayParameters{FolderID: ptr(st.FolderID), Name: ptr(names.Gateway()), SharedEgressGateway: []vpc.SharedEgressGatewayParameters{{}}}}}, k8slib.WithReady(yandexGatewayReady), failedOn[vpc.Gateway](), k8slib.WithResourceOption[vpc.Gateway](pipeline.Parent(sg)))
			routes := k8slib.Resource(ctx, k8s, names.RouteTable(), &vpc.RouteTable{Spec: vpc.RouteTableSpec{ResourceSpec: providerRef(pc), ForProvider: vpc.RouteTableParameters{FolderID: ptr(st.FolderID), Name: ptr(names.RouteTable()), NetworkID: netID, NetworkIDRef: netRef, StaticRoute: []vpc.StaticRouteParameters{{DestinationPrefix: ptr("0.0.0.0/0"), GatewayIDRef: &xpv1.Reference{Name: names.Gateway()}}}}}}, k8slib.WithReady(yandexRouteTableReady), failedOn[vpc.RouteTable](), k8slib.WithResourceOption[vpc.RouteTable](pipeline.Parent(gateway)))
			parent = routes
			live, err := routes.TryReady(ctx)
			if err != nil {
				return Infra{}, err
			}
			routeID = live.Status.AtProvider.ID
		}
	}
	for _, placement := range subnets {
		sub := k8slib.Resource(ctx, k8s, placement.Name, &vpc.Subnet{
			Spec: vpc.SubnetSpec{
				ResourceSpec: providerRef(pc),
				ForProvider: vpc.SubnetParameters{
					FolderID:     ptr(st.FolderID),
					Name:         ptr(placement.Name),
					NetworkID:    netID,
					NetworkIDRef: netRef,
					Zone:         ptr(placement.Zone),
					V4CidrBlocks: strPtrs(placement.CIDR),
					RouteTableID: routeID,
				},
			},
		}, k8slib.WithReady(yandexSubnetReady), failedOn[vpc.Subnet](), k8slib.WithResourceOption[vpc.Subnet](pipeline.Parent(parent)))
		subnetWaiters = append(subnetWaiters, func(ctx pipeline.Context) error { _, err := sub.TryReady(ctx); return err })
		subnetReady[placement.Zone] = readyOf(sub)
		if run.ManagedYDB != nil {
			parent = sub
		}

		byZone[placement.Zone] = sub
	}

	infra := Infra{Root: sg, Machines: map[string]*Machine{}}
	for _, m := range run.Machines {
		zone := m.Location
		if zone == "" {
			zone = st.Zone
		}
		sub := byZone[zone]
		subnetName := ""
		for _, placement := range subnets {
			if placement.Zone == zone {
				subnetName = placement.Name
				break
			}
		}
		agent, ok := agents[m.Name]
		if !ok {
			return Infra{}, fmt.Errorf("provision/yandex: no agent declared for machine %q", m.Name)
		}
		// Secondary disks are their own managed resources, attached by
		// reference; the boot disk is created inline from the image.
		var secondary []compute.SecondaryDiskParameters
		var diskHandles []pipeline.Handle
		for _, d := range m.Disks {
			diskName := names.Disk(m.Name, d.Name)
			disk := k8slib.Resource(ctx, k8s, diskName, &compute.Disk{
				Spec: compute.DiskSpec{
					ResourceSpec: providerRef(pc),
					ForProvider: compute.DiskParameters{
						FolderID:  ptr(st.FolderID),
						Name:      ptr(diskName),
						Zone:      ptr(zone),
						Size:      ptr(float64(d.GB)),
						BlockSize: ptr(float64(spec.DiskBlockSize(d.GB, d.BlockSize))),
						Type:      ptr(d.Type),
						Labels:    labels(run, m.Labels),
					},
				},
			}, k8slib.WithReady(yandexDiskReady), failedOn[compute.Disk](), k8slib.WithResourceOption[compute.Disk](pipeline.Parent(sub)))
			diskHandles = append(diskHandles, disk)
			secondary = append(secondary, compute.SecondaryDiskParameters{
				DeviceName: ptr(d.Name),
				DiskIDRef:  &xpv1.Reference{Name: diskName},
				AutoDelete: ptr(false), // the disk record deletes it; two deleters race
			})
		}
		bootGB, bootType := yandexBootDiskGB, "network-ssd"
		bootBlock := 0
		if m.BootDisk != nil {
			bootGB, bootType = m.BootDisk.GB, m.BootDisk.Type
			bootBlock = m.BootDisk.BlockSize
		}
		coreFraction := m.CoreFraction
		if coreFraction == 0 {
			coreFraction = 100
		}
		vmPublic, preemptible := public, st.Preemptible
		if m.PublicIP != nil {
			vmPublic = *m.PublicIP
		}
		if m.Preemptible != nil {
			preemptible = *m.Preemptible
		}
		vmName := names.Machine(m.Name)
		children := append([]pipeline.Handle{agent}, diskHandles...)
		vm := k8slib.Resource(ctx, k8s, vmName, &compute.Instance{
			Spec: compute.InstanceSpec{
				ResourceSpec: providerRef(pc),
				ForProvider: compute.InstanceParameters{
					FolderID: ptr(st.FolderID),
					Name:     ptr(vmName),
					// Unique in the cloud: runs share the profile network,
					// and a hostname must be unique inside a network.
					Hostname:   ptr(vmName),
					Zone:       ptr(zone),
					PlatformID: ptr(m.InstanceType),
					Resources: []compute.ResourcesParameters{{
						Cores:        ptr(float64(m.CPU)),
						CoreFraction: ptr(float64(coreFraction)),
						Memory:       ptr(float64(m.MemoryGB)),
					}},
					BootDisk: []compute.BootDiskParameters{{
						AutoDelete: ptr(true),
						InitializeParams: []compute.InitializeParamsParameters{{
							ImageID:   ptr(m.Image),
							Size:      ptr(float64(bootGB)),
							BlockSize: ptr(float64(spec.DiskBlockSize(bootGB, bootBlock))),
							Type:      ptr(bootType),
						}},
					}},
					SecondaryDisk: secondary,
					NetworkInterface: []compute.NetworkInterfaceParameters{{
						SubnetIDRef:          &xpv1.Reference{Name: subnetName},
						SecurityGroupIdsRefs: []xpv1.Reference{{Name: names.SecurityGroup()}},
						NAT:                  ptr(vmPublic),
					}},
					SchedulingPolicy: []compute.SchedulingPolicyParameters{{Preemptible: ptr(preemptible)}},
					Metadata: map[string]*string{
						"user-data":          ptr(agent.CloudInit()),
						"serial-port-enable": ptr("1"),
					},
					Labels: labels(run, m.Labels),
				},
			},
		},
			k8slib.WithReady(yandexInstanceReady), failedOn[compute.Instance](),
			k8slib.WithTimeout[compute.Instance](machineTimeout),
			k8slib.WithResourceOption[compute.Instance](pipeline.Parent(sub), pipeline.Children(children...)),
		)
		mach := &Machine{Spec: m, Agent: agent, VM: vm}
		deps := []readyFn{readyOf(sg)}
		if ready, ok := subnetReady[zone]; ok {
			deps = append(deps, ready)
		}
		mach.wait = func(ctx pipeline.Context) (MachineInfo, error) {
			if err := upstream(ctx, deps...); err != nil {
				return MachineInfo{}, err
			}
			live, err := vm.TryReady(ctx)
			if err != nil {
				return MachineInfo{}, err
			}
			return yandexInfo(live), nil
		}
		infra.Machines[m.Name] = mach
		infra.Order = append(infra.Order, m.Name)
	}
	if run.ManagedYDB != nil {
		infra.managedEndpoint = func(ctx pipeline.Context) (string, error) {
			group, err := sg.TryReady(ctx)
			if err != nil {
				return "", err
			}
			for _, wait := range subnetWaiters {
				if err := wait(ctx); err != nil {
					return "", err
				}
			}
			groupID := ""
			if group != nil && group.Status.AtProvider.ID != nil {
				groupID = *group.Status.AtProvider.ID
			}
			return yandexManagedEndpoint(ctx, k8s, run, st, names, subnets, groupID, parent, netRef, netID)
		}
	}
	return infra, nil
}

// yandexNetworkRef points a run object at the profile network: the
// Crossplane object of a created one by name, the user's network by id.
func yandexNetworkRef(network spec.YandexNetwork, providerConfig string) (ref *xpv1.Reference, id *string) {
	if network.Created() {
		return &xpv1.Reference{Name: spec.ProfileNetworkName(providerConfig)}, nil
	}
	return nil, ptr(network.NetworkID)
}

// yandexBootDiskGB is the boot disk of every VM; data lives on secondary
// disks or in the container volumes.
const yandexBootDiskGB = 40

// yandexIngress opens the run's own subnets to each other plus the spec's
// ingress. Other runs share the network but not these blocks, so they
// stay out.
func yandexIngress(run spec.Run, subnets []yandexSubnet) []vpc.SecurityGroupIngressParameters {
	blocks := make([]string, 0, len(subnets))
	for _, sub := range subnets {
		blocks = append(blocks, sub.CIDR)
	}
	rules := []vpc.SecurityGroupIngressParameters{{
		Description:  ptr("intra-run"),
		Protocol:     ptr("ANY"),
		FromPort:     ptr(0.0),
		ToPort:       ptr(65535.0),
		V4CidrBlocks: strPtrs(blocks...),
	}}
	// Dedicated YDB exposes gRPC through a managed network load balancer.
	// Its health checks originate outside the run subnet; allow only the
	// provider's health-check source on the database service port.
	if run.ManagedYDB != nil && run.ManagedYDB.Type == "dedicated" {
		rules = append(rules, vpc.SecurityGroupIngressParameters{
			Description:      ptr("managed YDB load balancer health checks"),
			Protocol:         ptr("TCP"),
			Port:             ptr(2135.0),
			PredefinedTarget: ptr("loadbalancer_healthchecks"),
		})
	}
	for _, in := range run.Network.Ingress {
		proto := "TCP"
		if in.Proto == "udp" {
			proto = "UDP"
		}
		rules = append(rules, vpc.SecurityGroupIngressParameters{
			Description:  ptr(fmt.Sprintf("ingress %d", in.Port)),
			Protocol:     ptr(proto),
			Port:         ptr(float64(in.Port)),
			V4CidrBlocks: strPtrs(in.CIDR),
		})
	}
	return rules
}

func yandexInfo(live *compute.Instance) MachineInfo {
	info := MachineInfo{}
	if live == nil {
		return info
	}
	if live.Status.AtProvider.ID != nil {
		info.ID = *live.Status.AtProvider.ID
	}
	for _, ni := range live.Status.AtProvider.NetworkInterface {
		if ni.IPAddress != nil && info.PrivateIP == "" {
			info.PrivateIP = *ni.IPAddress
		}
		if ni.NATIPAddress != nil && info.PublicIP == "" {
			info.PublicIP = *ni.NATIPAddress
		}
	}
	return info
}

func yandexNetworkReady(live *vpc.Network) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func yandexSubnetReady(live *vpc.Subnet) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func yandexSGReady(live *vpc.SecurityGroup) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func yandexDiskReady(live *compute.Disk) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

// yandexInstanceReady: Ready condition AND the instance runs — the
// condition alone flips true while the VM still boots.
func yandexInstanceReady(live *compute.Instance) bool {
	return xpReady(live.Status.GetCondition(xpv1.TypeReady)) &&
		live.Status.AtProvider.Status != nil && *live.Status.AtProvider.Status == "running"
}

func yandexGatewayReady(live *vpc.Gateway) bool {
	return live != nil && live.Status.AtProvider.ID != nil && xpReady(live.Status.GetCondition(xpv1.TypeReady))
}

func yandexRouteTableReady(live *vpc.RouteTable) bool {
	return live != nil && live.Status.AtProvider.ID != nil && xpReady(live.Status.GetCondition(xpv1.TypeReady))
}
