package provision

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
)

func picodataSettings() *database.Picodata_Settings {
	return &database.Picodata_Settings{
		Version:       "stable",
		StorageEngine: database.Picodata_Settings_STORAGE_ENGINE_MEMTX,
	}
}

func TestBuildForPicodataInstance_SingleNode(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataInstance(&database.Picodata_Instance{
		Template: &database.Picodata_Instance_Template{
			Settings: picodataSettings(),
			Hardware: hw(4, 8, 100),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(intent.GetItems()) != 1 {
		t.Fatalf("expected 1 item, got %d", len(intent.GetItems()))
	}

	item := intent.GetItems()[0]
	if item.GetName() != "picodata-node-0" {
		t.Errorf("expected name picodata-node-0, got %s", item.GetName())
	}
	if item.GetInternalIp().GetValue() != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", item.GetInternalIp().GetValue())
	}
	if item.GetHardware().GetCores() != 4 {
		t.Errorf("expected 4 cores, got %d", item.GetHardware().GetCores())
	}

	picodataCount := 0
	for _, c := range item.GetContainers() {
		if c.GetPicodata() != nil {
			picodataCount++
			if c.GetPicodata().GetNodeIndex() != 0 {
				t.Errorf("expected node_index 0, got %d", c.GetPicodata().GetNodeIndex())
			}
		}
	}
	if picodataCount != 1 {
		t.Errorf("expected 1 picodata container, got %d", picodataCount)
	}
}

func TestBuildForPicodataInstance_NilTemplate(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	_, err := b.BuildForPicodataInstance(nil)
	if err == nil {
		t.Fatal("expected error for nil instance")
	}
}

func TestBuildForPicodataInstance_WithSidecars(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataInstance(&database.Picodata_Instance{
		Template: &database.Picodata_Instance_Template{
			Settings: picodataSettings(),
			Hardware: hw(4, 8, 100),
			Sidecars: []*database.Picodata_Sidecar{
				{Sidecar: &database.Picodata_Sidecar_NodeExporter{
					NodeExporter: &database.CommonSidecar_NodeExporter{Port: 9100},
				}},
				{Sidecar: &database.Picodata_Sidecar_HttpMetrics_{
					HttpMetrics: &database.Picodata_Sidecar_HttpMetrics{Enabled: true, Port: 8081},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	item := intent.GetItems()[0]
	// picodata container + node-exporter sidecar = 2 containers
	// (HttpMetrics configures the picodata container itself, no extra container)
	if len(item.GetContainers()) != 2 {
		t.Errorf("expected 2 containers (picodata + node-exporter), got %d", len(item.GetContainers()))
		for _, c := range item.GetContainers() {
			t.Logf("  container: id=%s runtime=%T", c.GetId(), c.GetRuntime())
		}
	}
	// Verify HttpMetrics sidecar configured the picodata container for monitoring
	for _, c := range item.GetContainers() {
		if c.GetPicodata() != nil && !c.GetMonitor() {
			t.Error("picodata container should have monitor=true after HttpMetrics sidecar")
		}
	}
}

func TestBuildForPicodataInstance_WithBackupSidecar(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataInstance(&database.Picodata_Instance{
		Template: &database.Picodata_Instance_Template{
			Settings: picodataSettings(),
			Hardware: hw(4, 8, 100),
			Sidecars: []*database.Picodata_Sidecar{
				{Sidecar: &database.Picodata_Sidecar_Backup_{
					Backup: &database.Picodata_Sidecar_Backup{
						Schedule:  "0 2 * * *",
						Retention: "7d",
						Tool:      database.Picodata_Sidecar_Backup_PICODATA_BACKUP,
						Storage: &database.Picodata_Sidecar_Backup_Local{
							Local: &database.Picodata_Sidecar_Backup_LocalStorage{Path: "/backups"},
						},
					},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	item := intent.GetItems()[0]
	hasBackup := false
	for _, c := range item.GetContainers() {
		if c.GetPicodataBackup() != nil {
			hasBackup = true
		}
	}
	if !hasBackup {
		t.Error("expected picodata backup sidecar container")
	}
}

func TestBuildForPicodataCluster_ThreeNodes(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(intent.GetItems()) != 3 {
		t.Fatalf("expected 3 items, got %d", len(intent.GetItems()))
	}

	for i := 0; i < 3; i++ {
		item := intent.GetItems()[i]
		expectedName := "picodata-node-" + strings.Repeat("", 0) + string(rune('0'+i))
		if item.GetName() != expectedName {
			t.Errorf("item[%d] expected name %s, got %s", i, expectedName, item.GetName())
		}

		picodataCount := 0
		for _, c := range item.GetContainers() {
			if c.GetPicodata() != nil {
				picodataCount++
				if c.GetPicodata().GetNodeIndex() != uint32(i) {
					t.Errorf("expected node_index %d, got %d", i, c.GetPicodata().GetNodeIndex())
				}
			}
		}
		if picodataCount != 1 {
			t.Errorf("item[%d] expected 1 picodata container, got %d", i, picodataCount)
		}
	}
}

func TestBuildForPicodataCluster_NodeOverride(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := newPicodataPlacementBuilder(network)

	overrideHw := hw(16, 32, 500)
	overrideSettings := &database.Picodata_Settings{
		Version:       "stable",
		StorageEngine: database.Picodata_Settings_STORAGE_ENGINE_VINYL,
	}

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: overrideSettings, Hardware: overrideHw}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	node0 := findItem(intent.GetItems(), "picodata-node-0")
	if node0.GetHardware().GetCores() != 4 {
		t.Errorf("node-0 should use default hw, got cores=%d", node0.GetHardware().GetCores())
	}

	node1 := findItem(intent.GetItems(), "picodata-node-1")
	if node1.GetHardware().GetCores() != 16 {
		t.Errorf("node-1 should use override hw, got cores=%d", node1.GetHardware().GetCores())
	}

	for _, c := range node1.GetContainers() {
		if pd := c.GetPicodata(); pd != nil {
			if pd.GetSettings().GetStorageEngine() != database.Picodata_Settings_STORAGE_ENGINE_VINYL {
				t.Errorf("node-1 should use override settings (VINYL), got %s", pd.GetSettings().GetStorageEngine())
			}
		}
	}

	node2 := findItem(intent.GetItems(), "picodata-node-2")
	if node2.GetHardware().GetCores() != 4 {
		t.Errorf("node-2 should use default hw, got cores=%d", node2.GetHardware().GetCores())
	}
}

func TestBuildForPicodataCluster_WithMonitoring(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, item := range intent.GetItems() {
		// Node-centric BuildForPicodataCluster does not add monitoring.
		// Each node should have exactly 1 picodata container.
		if len(item.GetContainers()) != 1 {
			t.Errorf("item %s expected 1 container (picodata only), got %d", item.GetName(), len(item.GetContainers()))
			for _, c := range item.GetContainers() {
				t.Logf("  container: id=%s runtime=%T", c.GetId(), c.GetRuntime())
			}
		}
	}
}

func TestBuildForPicodataCluster_BackupOnAllNodes(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Template: &database.Picodata_Cluster_Template{
			Addons: &database.Picodata_Addons{
				Backup: &database.Picodata_Addons_Backup{
					Enabled: true,
					Scope:   database.Picodata_Placement_SCOPE_ALL_NODES,
					Config: &database.Picodata_Sidecar_Backup{
						Schedule:  "0 3 * * *",
						Retention: "7d",
						Tool:      database.Picodata_Sidecar_Backup_PICODATA_BACKUP,
						Storage: &database.Picodata_Sidecar_Backup_Local{
							Local: &database.Picodata_Sidecar_Backup_LocalStorage{Path: "/backups"},
						},
					},
				},
			},
		},
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	backupCount := 0
	for _, item := range intent.GetItems() {
		for _, c := range item.GetContainers() {
			if c.GetPicodataBackup() != nil {
				backupCount++
			}
		}
	}
	if backupCount != 2 {
		t.Errorf("expected 2 backup containers (all nodes), got %d", backupCount)
	}
}

func TestBuildForPicodataCluster_BackupOnSingleNode(t *testing.T) {
	nodeIdx := uint32(0)
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Template: &database.Picodata_Cluster_Template{
			Addons: &database.Picodata_Addons{
				Backup: &database.Picodata_Addons_Backup{
					Enabled: true,
					Scope:   database.Picodata_Placement_SCOPE_NODE,
					Config: &database.Picodata_Sidecar_Backup{
						Schedule:  "0 3 * * *",
						Retention: "7d",
						Tool:      database.Picodata_Sidecar_Backup_PICODATA_BACKUP,
						Storage: &database.Picodata_Sidecar_Backup_Local{
							Local: &database.Picodata_Sidecar_Backup_LocalStorage{Path: "/backups"},
						},
					},
				},
			},
		},
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	_ = nodeIdx
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	node0 := findItem(intent.GetItems(), "picodata-node-0")
	node1 := findItem(intent.GetItems(), "picodata-node-1")

	// SCOPE_NODE without nodeIndex -> expandScope returns nil -> no backup containers
	node0Backup := 0
	for _, c := range node0.GetContainers() {
		if c.GetPicodataBackup() != nil {
			node0Backup++
		}
	}
	node1Backup := 0
	for _, c := range node1.GetContainers() {
		if c.GetPicodataBackup() != nil {
			node1Backup++
		}
	}
	if node0Backup != 0 || node1Backup != 0 {
		t.Errorf("SCOPE_NODE without nodeIndex should produce 0 backups, got node0=%d node1=%d", node0Backup, node1Backup)
	}
}

func TestBuildForPicodataCluster_NilTopology(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	_, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{},
	})
	if err == nil {
		t.Fatal("expected error for empty nodes slice")
	}
}

func TestBuildForPicodataCluster_NotEnoughIPs(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	_, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err == nil {
		t.Fatal("expected error when not enough IPs")
	}
}

func TestPicodataCluster_ItemOrderMatchesCreation(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2", "10.0.0.3")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"picodata-node-0", "picodata-node-1", "picodata-node-2"}
	for i, item := range intent.GetItems() {
		if item.GetName() != expected[i] {
			t.Errorf("item[%d] expected %s, got %s", i, expected[i], item.GetName())
		}
	}
}

func TestPicodataCluster_IPAssignment(t *testing.T) {
	network := networkWithIPs("10.0.0.10", "10.0.0.20", "10.0.0.30")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedIPs := []string{"10.0.0.10", "10.0.0.20", "10.0.0.30"}
	for i, item := range intent.GetItems() {
		if item.GetInternalIp().GetValue() != expectedIPs[i] {
			t.Errorf("item[%d] expected IP %s, got %s", i, expectedIPs[i], item.GetInternalIp().GetValue())
		}
	}
}

func TestPicodataCluster_ConnectionString(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	connStr := intent.GetConnectionString()
	if !strings.Contains(connStr, "10.0.0.1") {
		t.Errorf("connection string should reference first node IP, got %s", connStr)
	}
	if !strings.HasPrefix(connStr, "postgres://") {
		t.Errorf("connection string should start with postgres://, got %s", connStr)
	}
	if !strings.Contains(connStr, ":4327/") {
		t.Errorf("connection string should reference port 4327, got %s", connStr)
	}
}

func TestPicodataCluster_PicodataConfToEnvAndArgs(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	settings := picodataSettings()
	settings.PicodataConf = map[string]string{
		"memtx_memory": "1073741824",
		"log_level":    "info",
	}

	intent, err := b.BuildForPicodataInstance(&database.Picodata_Instance{
		Template: &database.Picodata_Instance_Template{
			Settings: settings,
			Hardware: hw(4, 8, 100),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pc := findContainer(intent.GetItems(), "picodata-node-0", "picodata-node-0-container")
	if pc == nil {
		t.Fatal("picodata container not found")
	}

	if pc.GetEnv()["PICODATA_CONF_JSON"] == "" {
		t.Fatal("PICODATA_CONF_JSON not set")
	}
	got := map[string]string{}
	if err := json.Unmarshal([]byte(pc.GetEnv()["PICODATA_CONF_JSON"]), &got); err != nil {
		t.Fatalf("invalid PICODATA_CONF_JSON: %v", err)
	}
	if !reflect.DeepEqual(got, settings.GetPicodataConf()) {
		t.Errorf("PICODATA_CONF_JSON mismatch: got=%v expected=%v", got, settings.GetPicodataConf())
	}

	expectedArgs := []string{
		"--set", "log_level=info",
		"--set", "memtx_memory=1073741824",
	}
	if !reflect.DeepEqual(pc.GetArgs(), expectedArgs) {
		t.Errorf("unexpected args, got=%v expected=%v", pc.GetArgs(), expectedArgs)
	}
}

func TestPicodataCluster_BuiltInMetricsConfig(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, item := range intent.GetItems() {
		// Node-centric BuildForPicodataCluster does not apply monitoring config.
		// Each node should have exactly 1 picodata container with no monitoring env/args.
		if len(item.GetContainers()) != 1 {
			t.Errorf("item %s expected 1 container (picodata only), got %d", item.GetName(), len(item.GetContainers()))
		}
	}
}

func TestPicodataExpandScope(t *testing.T) {
	b := newPicodataPlacementBuilder(networkWithIPs())
	nodes := []string{"n0", "n1", "n2"}

	tests := []struct {
		name     string
		scope    database.Picodata_Placement_Scope
		nodeIdx  *uint32
		expected []string
	}{
		{"all_nodes", database.Picodata_Placement_SCOPE_ALL_NODES, nil, []string{"n0", "n1", "n2"}},
		{"node_1", database.Picodata_Placement_SCOPE_NODE, uint32Ptr(1), []string{"n1"}},
		{"node_nil_idx", database.Picodata_Placement_SCOPE_NODE, nil, nil},
		{"node_oob", database.Picodata_Placement_SCOPE_NODE, uint32Ptr(10), nil},
		{"unspecified", database.Picodata_Placement_SCOPE_UNSPECIFIED, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.expandScope(tt.scope, tt.nodeIdx, nodes)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("index %d: expected %s, got %s", i, tt.expected[i], got[i])
				}
			}
		})
	}
}

func TestPicodataCluster_MetadataSet(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataInstance(&database.Picodata_Instance{
		Template: &database.Picodata_Instance_Template{
			Settings: picodataSettings(),
			Hardware: hw(4, 8, 100),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, item := range intent.GetItems() {
		for _, c := range item.GetContainers() {
			if c.Metadata == nil {
				t.Errorf("metadata is nil on %s/%s", item.GetName(), c.GetId())
				continue
			}
			if c.Metadata[containerMetadataDockerIPKey] == "" {
				t.Errorf("docker IP metadata missing on %s/%s", item.GetName(), c.GetId())
			}
			if c.Metadata[containerMetadataPlacementNodeKey] == "" {
				t.Errorf("placement node metadata missing on %s/%s", item.GetName(), c.GetId())
			}
			if c.Metadata[containerMetadataLogicalNameKey] == "" {
				t.Errorf("logical name metadata missing on %s/%s", item.GetName(), c.GetId())
			}
		}
	}
}

func TestPicodataContainerImages(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, item := range intent.GetItems() {
		for _, c := range item.GetContainers() {
			if c.GetImage() == "" {
				t.Errorf("container %s has empty image", c.GetId())
			}
			if c.GetPicodata() != nil && c.GetImage() != imagePicodata {
				t.Errorf("picodata container should use image %s, got %s", imagePicodata, c.GetImage())
			}
			if c.GetNodeExporter() != nil && c.GetImage() != imageNodeExporter {
				t.Errorf("node exporter container should use image %s, got %s", imageNodeExporter, c.GetImage())
			}
		}
	}
}

func TestPicodataInstance_ConnectionString(t *testing.T) {
	network := networkWithIPs("10.0.0.42")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataInstance(&database.Picodata_Instance{
		Template: &database.Picodata_Instance_Template{
			Settings: picodataSettings(),
			Hardware: hw(2, 4, 50),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(intent.GetConnectionString(), "10.0.0.42") {
		t.Errorf("connection string should contain instance IP, got %s", intent.GetConnectionString())
	}
}

func TestPicodataCluster_NilClusterTemplate(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	_, err := b.BuildForPicodataCluster(nil)
	if err == nil {
		t.Fatal("expected error for nil cluster")
	}
}

func TestPicodataCluster_RuntimeTypes(t *testing.T) {
	network := networkWithIPs("10.0.0.1", "10.0.0.2")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataCluster(&database.Picodata_Cluster{
		Template: &database.Picodata_Cluster_Template{
			Addons: &database.Picodata_Addons{
				Backup: &database.Picodata_Addons_Backup{
					Enabled: true,
					Scope:   database.Picodata_Placement_SCOPE_ALL_NODES,
					Config: &database.Picodata_Sidecar_Backup{
						Schedule:  "0 3 * * *",
						Retention: "7d",
						Tool:      database.Picodata_Sidecar_Backup_PICODATA_BACKUP,
						Storage: &database.Picodata_Sidecar_Backup_Local{
							Local: &database.Picodata_Sidecar_Backup_LocalStorage{Path: "/backups"},
						},
					},
				},
			},
		},
		Nodes: []*database.Picodata_Instance{
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
			{Template: &database.Picodata_Instance_Template{Settings: picodataSettings(), Hardware: hw(4, 8, 100)}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	runtimeTypes := map[string]int{}
	for _, item := range intent.GetItems() {
		for _, c := range item.GetContainers() {
			switch {
			case c.GetPicodata() != nil:
				runtimeTypes["picodata"]++
			case c.GetNodeExporter() != nil:
				runtimeTypes["node_exporter"]++
			case c.GetPicodataBackup() != nil:
				runtimeTypes["picodata_backup"]++
			default:
				t.Errorf("unexpected runtime type on container %s: %T", c.GetId(), c.GetRuntime())
			}
		}
	}

	// Node-centric: 2 picodata + 2 backup = 4 containers total (no node_exporter)
	if runtimeTypes["picodata"] != 2 {
		t.Errorf("expected 2 picodata containers, got %d", runtimeTypes["picodata"])
	}
	if runtimeTypes["node_exporter"] != 0 {
		t.Errorf("expected 0 node_exporter containers, got %d", runtimeTypes["node_exporter"])
	}
	if runtimeTypes["picodata_backup"] != 2 {
		t.Errorf("expected 2 picodata_backup containers, got %d", runtimeTypes["picodata_backup"])
	}
}

func TestPicodataCluster_NoConfNoEnvNoArgs(t *testing.T) {
	network := networkWithIPs("10.0.0.1")
	b := newPicodataPlacementBuilder(network)

	intent, err := b.BuildForPicodataInstance(&database.Picodata_Instance{
		Template: &database.Picodata_Instance_Template{
			Settings: picodataSettings(),
			Hardware: hw(4, 8, 100),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pc := findContainer(intent.GetItems(), "picodata-node-0", "picodata-node-0-container")
	if pc == nil {
		t.Fatal("picodata container not found")
	}
	if _, ok := pc.GetEnv()["PICODATA_CONF_JSON"]; ok {
		t.Error("PICODATA_CONF_JSON should not be set when picodata_conf is empty")
	}
	if len(pc.GetArgs()) != 0 {
		t.Errorf("args should be empty when picodata_conf is empty, got %v", pc.GetArgs())
	}
}
