package provision

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
)

const (
	imagePicodata       = "docker.binary.picodata.io/picodata:latest"
	imagePicodataBackup = "docker.binary.picodata.io/picodata:latest"

	defaultPortPicodata     uint32 = 4327
	defaultPortPicodataHTTP uint32 = 8081
)

type picodataPlacementBuilder struct {
	itemsByName map[string]*provision.PlacementIntent_Item
	nodeOrder   []string
	network     *deployment.Network
}

func newPicodataPlacementBuilder(network *deployment.Network) *picodataPlacementBuilder {
	return &picodataPlacementBuilder{
		itemsByName: map[string]*provision.PlacementIntent_Item{},
		nodeOrder:   make([]string, 0),
		network:     network,
	}
}

func (p *picodataPlacementBuilder) BuildForPicodataInstance(
	inst *database.Picodata_Instance,
) (*provision.PlacementIntent, error) {
	if inst == nil {
		return nil, fmt.Errorf("picodata instance is nil")
	}
	tmpl := inst.GetTemplate()
	if tmpl == nil {
		return nil, fmt.Errorf("picodata instance template is nil")
	}
	if err := p.addInstanceTemplate(tmpl); err != nil {
		return nil, err
	}
	return p.finalize()
}

func (p *picodataPlacementBuilder) BuildForPicodataCluster(
	cluster *database.Picodata_Cluster,
) (*provision.PlacementIntent, error) {
	if cluster == nil {
		return nil, fmt.Errorf("picodata cluster is nil")
	}
	nodes := cluster.GetNodes()
	if len(nodes) == 0 {
		return nil, fmt.Errorf("picodata cluster must have at least 1 node")
	}
	for i, node := range nodes {
		if node == nil {
			continue
		}
		tmpl := node.GetTemplate()
		if tmpl == nil {
			return nil, fmt.Errorf("picodata node %d has no template", i)
		}
		name := fmt.Sprintf("picodata-node-%d", i)
		item, err := p.ensureItem(name, tmpl.GetHardware())
		if err != nil {
			return nil, err
		}
		item.Containers = append(item.Containers, p.newPicodataContainer(
			name, name, tmpl.GetSettings(), uint32(i), false,
		))
		p.addSidecarsToItem(item, node.GetSidecars())
	}

	if cluster.GetTemplate() != nil {
		nodeNames := make([]string, len(nodes))
		for i := range nodes {
			nodeNames[i] = fmt.Sprintf("picodata-node-%d", i)
		}
		p.addBackupAddons(cluster.GetTemplate().GetAddons().GetBackup(), nodeNames)
	}
	return p.finalize()
}

func (p *picodataPlacementBuilder) finalize() (*provision.PlacementIntent, error) {
	items, err := p.finalizeItems()
	if err != nil {
		return nil, err
	}
	connStr, err := p.resolveRuntimeConfig(items)
	if err != nil {
		return nil, err
	}
	return &provision.PlacementIntent{
		Items:            items,
		Network:          p.network,
		ConnectionString: connStr,
	}, nil
}

func (p *picodataPlacementBuilder) addInstanceTemplate(tmpl *database.Picodata_Instance_Template) error {
	if tmpl == nil {
		return fmt.Errorf("picodata instance template is nil")
	}
	item, err := p.ensureItem("picodata-node-0", tmpl.GetHardware())
	if err != nil {
		return err
	}
	item.Containers = append(item.Containers, p.newPicodataContainer(
		"picodata-node-0",
		"picodata-node-0",
		tmpl.GetSettings(),
		0,
		false,
	))
	p.addSidecarsToItem(item, tmpl.GetSidecars())
	return nil
}

func (p *picodataPlacementBuilder) addBackupAddons(
	backup *database.Picodata_Addons_Backup,
	nodeNames []string,
) []string {
	if backup == nil || !backup.GetEnabled() || backup.GetConfig() == nil {
		return nil
	}
	targets := p.expandScope(backup.GetScope(), nil, nodeNames)
	names := make([]string, 0, len(targets))
	for i, name := range targets {
		item := p.itemsByName[name]
		item.Containers = append(item.Containers, &provision.Container{
			Id:      "picodata-backup-" + strconv.Itoa(i+1),
			Name:    "picodata-backup-" + strconv.Itoa(i+1),
			Image:   imagePicodataBackup,
			Monitor: false,
			Runtime: &provision.Container_PicodataBackup{
				PicodataBackup: &provision.Container_PicodataBackupRuntime{
					Config: backup.GetConfig(),
				},
			},
		})
		names = append(names, name)
	}
	return names
}

// addPicodataMonitoring adds OS-level node-exporter only.
// DB-level metrics are served by Picodata's built-in HTTP endpoint
// (configured via --http-listen in resolveRuntimeConfig).
func (p *picodataPlacementBuilder) addPicodataMonitoring(item *provision.PlacementIntent_Item, suffix string) {
	item.Containers = append(item.Containers, NewNodeExporterContainer(suffix, true))
}

func (p *picodataPlacementBuilder) addSidecarsToItem(item *provision.PlacementIntent_Item, sidecars []*database.Picodata_Sidecar) {
	for i, s := range sidecars {
		if s == nil {
			continue
		}
		if ne := s.GetNodeExporter(); ne != nil {
			port := ne.GetPort()
			if port == 0 {
				port = defaultPortNodeExporter
			}
			item.Containers = append(item.Containers, &provision.Container{
				Id:      "node-exporter-sidecar-" + strconv.Itoa(i),
				Name:    "node-exporter-sidecar-" + strconv.Itoa(i),
				Image:   imageNodeExporter,
				Monitor: true,
				Runtime: &provision.Container_NodeExporter{
					NodeExporter: &provision.Container_NodeExporterRuntime{
						Enabled: true,
						Port:    port,
					},
				},
			})
		}
		if hm := s.GetHttpMetrics(); hm != nil && hm.GetEnabled() {
			port := hm.GetPort()
			if port == 0 {
				port = defaultPortPicodataHTTP
			}
			for _, c := range item.GetContainers() {
				if c.GetPicodata() != nil {
					c.Monitor = true
					ensureEnv(c)
					c.Env["PICODATA_HTTP_LISTEN"] = fmt.Sprintf("0.0.0.0:%d", port)
				}
			}
		}
		if b := s.GetBackup(); b != nil {
			item.Containers = append(item.Containers, &provision.Container{
				Id:      "picodata-backup-sidecar-" + strconv.Itoa(i),
				Name:    "picodata-backup-sidecar-" + strconv.Itoa(i),
				Image:   imagePicodataBackup,
				Monitor: false,
				Runtime: &provision.Container_PicodataBackup{
					PicodataBackup: &provision.Container_PicodataBackupRuntime{
						Config: b,
					},
				},
			})
		}
	}
}

func (p *picodataPlacementBuilder) newPicodataContainer(
	id string,
	name string,
	settings *database.Picodata_Settings,
	nodeIndex uint32,
	monitor bool,
) *provision.Container {
	return &provision.Container{
		Id:      id + "-container",
		Name:    name + "-container",
		Image:   imagePicodata,
		Monitor: monitor,
		Runtime: &provision.Container_Picodata{
			Picodata: &provision.Container_PicodataRuntime{
				NodeIndex: nodeIndex,
				Settings:  settings,
			},
		},
	}
}

func (p *picodataPlacementBuilder) expandPlacement(
	placement *database.Picodata_Placement,
	nodeNames []string,
) ([]string, error) {
	if placement == nil {
		return nil, nil
	}
	switch mode := placement.GetMode().(type) {
	case *database.Picodata_Placement_Colocate_:
		return p.expandScope(mode.Colocate.GetScope(), mode.Colocate.NodeIndex, nodeNames), nil
	case *database.Picodata_Placement_Dedicated_:
		n := int(mode.Dedicated.GetInstancesCount())
		names := make([]string, 0, n)
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("picodata-dedicated-%d", len(p.nodeOrder)+1)
			if _, err := p.ensureItem(name, mode.Dedicated.GetHardware()); err != nil {
				return nil, err
			}
			names = append(names, name)
		}
		return names, nil
	default:
		return nil, fmt.Errorf("placement mode is not set")
	}
}

func (p *picodataPlacementBuilder) expandScope(
	scope database.Picodata_Placement_Scope,
	nodeIndex *uint32,
	nodeNames []string,
) []string {
	switch scope {
	case database.Picodata_Placement_SCOPE_ALL_NODES:
		return append([]string{}, nodeNames...)
	case database.Picodata_Placement_SCOPE_NODE:
		if nodeIndex == nil {
			return nil
		}
		idx := int(*nodeIndex)
		if idx < 0 || idx >= len(nodeNames) {
			return nil
		}
		return []string{nodeNames[idx]}
	default:
		return nil
	}
}

func (p *picodataPlacementBuilder) ensureItem(name string, hw *deployment.Hardware) (*provision.PlacementIntent_Item, error) {
	if hw == nil {
		return nil, fmt.Errorf("hardware is required for item %q", name)
	}
	if item, ok := p.itemsByName[name]; ok {
		return item, nil
	}
	item := &provision.PlacementIntent_Item{
		Name:       name,
		Hardware:   hw,
		Containers: make([]*provision.Container, 0),
	}
	p.itemsByName[name] = item
	p.nodeOrder = append(p.nodeOrder, name)
	return item, nil
}

func (p *picodataPlacementBuilder) finalizeItems() ([]*provision.PlacementIntent_Item, error) {
	items := make([]*provision.PlacementIntent_Item, 0, len(p.nodeOrder))
	ips := p.network.GetIps()
	if len(ips) < len(p.nodeOrder) {
		return nil, fmt.Errorf("network has %d ips, but %d items are required", len(ips), len(p.nodeOrder))
	}
	for i, name := range p.nodeOrder {
		item := p.itemsByName[name]
		if ips[i] == nil || ips[i].GetValue() == "" {
			return nil, fmt.Errorf("network ip at index %d is empty", i)
		}
		item.InternalIp = ips[i]
		for _, c := range item.GetContainers() {
			ensureMetadata(c)
			c.Metadata[containerMetadataDockerIPKey] = item.GetInternalIp().GetValue()
			c.Metadata[containerMetadataPlacementNodeKey] = item.GetName()
			c.Metadata[containerMetadataLogicalNameKey] = containerLogicalName(c)
		}
		items = append(items, item)
	}
	return items, nil
}

func (p *picodataPlacementBuilder) resolveRuntimeConfig(items []*provision.PlacementIntent_Item) (string, error) {
	var firstNodeIP string
	for _, item := range items {
		itemIP := item.GetInternalIp().GetValue()
		for _, c := range item.GetContainers() {
			if c.GetPicodata() == nil {
				continue
			}
			if firstNodeIP == "" {
				firstNodeIP = itemIP
			}
			if err := applyPicodataConf(c, c.GetPicodata().GetSettings()); err != nil {
				return "", err
			}
			if c.GetMonitor() {
				httpPort := defaultPortPicodataHTTP
				if envPort := c.GetEnv()["PICODATA_HTTP_LISTEN"]; envPort != "" {
					httpPort = 0 // already configured by sidecar
				}
				if httpPort != 0 {
					ensureEnv(c)
					c.Env["PICODATA_HTTP_LISTEN"] = fmt.Sprintf("0.0.0.0:%d", httpPort)
				}
				c.Args = append(c.Args, "--http-listen", c.GetEnv()["PICODATA_HTTP_LISTEN"])
				c.Ports = append(c.Ports, &provision.ContainerPort{
					Name:          "http-metrics",
					ContainerPort: defaultPortPicodataHTTP,
				})
			}
		}
	}
	if firstNodeIP == "" {
		return "", fmt.Errorf("picodata node not found in placement items")
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		defaultPostgresUser,
		defaultPostgresPassword,
		firstNodeIP,
		defaultPortPicodata,
		defaultPostgresDatabase,
	), nil
}

func applyPicodataConf(c *provision.Container, settings *database.Picodata_Settings) error {
	if c == nil || settings == nil || len(settings.GetPicodataConf()) == 0 {
		return nil
	}
	ensureEnv(c)
	encoded, err := json.Marshal(settings.GetPicodataConf())
	if err != nil {
		return fmt.Errorf("encode picodata_conf for %q: %w", c.GetName(), err)
	}
	c.Env["PICODATA_CONF_JSON"] = string(encoded)

	c.Args = append(c.Args, picodataConfArgs(settings.GetPicodataConf())...)
	return nil
}

func picodataConfArgs(conf map[string]string) []string {
	keys := make([]string, 0, len(conf))
	for k := range conf {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(conf)*2)
	for _, k := range keys {
		args = append(args, "--set", fmt.Sprintf("%s=%s", k, conf[k]))
	}
	return args
}
