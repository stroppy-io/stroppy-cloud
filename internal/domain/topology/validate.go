package topology

import (
	"context"
	"fmt"
	"regexp"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
)

var nodeNameRegexp = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func ValidateDatabaseTemplate(ctx context.Context, template *database.Database_Template) error {
	_ = ctx
	if template == nil {
		return fmt.Errorf("database template is nil")
	}

	switch t := template.Template.(type) {
	case *database.Database_Template_PostgresInstance:
		if err := template.Validate(); err != nil {
			return err
		}
		return validatePostgresInstance(t.PostgresInstance)
	case *database.Database_Template_PostgresCluster:
		if err := template.Validate(); err != nil {
			return err
		}
		return validatePostgresCluster(t.PostgresCluster)
	case *database.Database_Template_PicodataInstance:
		if err := template.Validate(); err != nil {
			return err
		}
		return validatePicodataInstance(t.PicodataInstance)
	case *database.Database_Template_PicodataCluster:
		// Skip generic proto Validate() for Picodata clusters: the proto requires
		// nodes (min_items=1), but nodes are materialized from the template at
		// deployment time and are empty during template editing/validation.
		return validatePicodataCluster(t.PicodataCluster)
	case nil:
		return fmt.Errorf("database template content is nil")
	default:
		return fmt.Errorf("unknown database template type")
	}
}

func validatePicodataInstance(inst *database.Picodata_Instance) error {
	if inst == nil {
		return fmt.Errorf("picodata instance is nil")
	}
	tmpl := inst.GetTemplate()
	if tmpl == nil {
		return fmt.Errorf("picodata instance template is nil")
	}
	s := tmpl.GetSettings()
	if s == nil {
		return fmt.Errorf("picodata instance settings is nil")
	}
	if s.GetVersion() == "" {
		return fmt.Errorf("picodata version is required")
	}
	if tmpl.GetHardware() == nil {
		return fmt.Errorf("picodata instance hardware is required")
	}
	return nil
}

func validatePicodataCluster(cluster *database.Picodata_Cluster) error {
	if cluster == nil {
		return fmt.Errorf("picodata cluster is nil")
	}
	tmpl := cluster.GetTemplate()
	if tmpl == nil {
		return fmt.Errorf("picodata cluster template is nil")
	}
	topo := tmpl.GetTopology()
	if topo == nil {
		return fmt.Errorf("picodata cluster topology is nil")
	}
	if topo.GetNodesCount() < 1 {
		return fmt.Errorf("picodata cluster must have at least 1 node")
	}
	s := topo.GetSettings()
	if s == nil {
		return fmt.Errorf("picodata cluster settings is nil")
	}
	if s.GetVersion() == "" {
		return fmt.Errorf("picodata version is required")
	}
	if topo.GetNodeHardware() == nil {
		return fmt.Errorf("picodata cluster node hardware is required")
	}
	return nil
}

func validatePostgresInstance(inst *database.Postgres_Instance) error {
	if inst == nil {
		return fmt.Errorf("postgres instance is nil")
	}
	if err := validateSettings(inst.GetDefaults()); err != nil {
		return err
	}
	if patroni := inst.GetDefaults().GetPatroni(); patroni != nil && patroni.GetEnabled() {
		return fmt.Errorf("patroni is only supported for postgres cluster")
	}

	node := inst.GetNode()
	if node == nil {
		return fmt.Errorf("instance node is nil")
	}
	if err := validateNode(node); err != nil {
		return fmt.Errorf("instance node: %w", err)
	}
	if node.GetPostgres() == nil {
		return fmt.Errorf("instance node must have a postgres service")
	}
	if node.GetPostgres().GetRole() != database.Postgres_PostgresService_ROLE_MASTER {
		return fmt.Errorf("instance node postgres role must be ROLE_MASTER")
	}
	if node.GetEtcd() != nil {
		return fmt.Errorf("etcd is not supported on a standalone instance")
	}
	if node.GetPostgres().GetSettings() != nil {
		if err := validateSettings(node.GetPostgres().GetSettings()); err != nil {
			return fmt.Errorf("instance node postgres settings: %w", err)
		}
	}
	return nil
}

func validatePostgresCluster(cluster *database.Postgres_Cluster) error {
	if cluster == nil {
		return fmt.Errorf("postgres cluster is nil")
	}
	if err := validateSettings(cluster.GetDefaults()); err != nil {
		return err
	}

	nodes := cluster.GetNodes()
	if len(nodes) < 2 {
		return fmt.Errorf("cluster must have at least 2 nodes")
	}

	seenNames := make(map[string]struct{})
	masterCount := 0
	replicaCount := 0
	etcdCount := 0

	for i, node := range nodes {
		if node == nil {
			return fmt.Errorf("node at index %d is nil", i)
		}
		if err := validateNode(node); err != nil {
			return fmt.Errorf("node %q: %w", node.GetName(), err)
		}
		name := node.GetName()
		if _, exists := seenNames[name]; exists {
			return fmt.Errorf("duplicate node name %q", name)
		}
		seenNames[name] = struct{}{}

		if pg := node.GetPostgres(); pg != nil {
			switch pg.GetRole() {
			case database.Postgres_PostgresService_ROLE_MASTER:
				masterCount++
			case database.Postgres_PostgresService_ROLE_REPLICA:
				replicaCount++
			}
			if pg.GetSettings() != nil {
				if err := validateSettings(pg.GetSettings()); err != nil {
					return fmt.Errorf("node %q postgres settings: %w", name, err)
				}
			}
		}

		if node.GetEtcd() != nil {
			etcdCount++
		}
	}

	if masterCount != 1 {
		return fmt.Errorf("cluster must have exactly 1 master node, got %d", masterCount)
	}
	if replicaCount < 1 {
		return fmt.Errorf("cluster must have at least 1 replica node")
	}

	if etcdCount > 0 {
		switch etcdCount {
		case 1, 3, 5:
			// valid quorum sizes
		default:
			return fmt.Errorf("etcd node count must be 1, 3, or 5, got %d", etcdCount)
		}
	}

	patroni := cluster.GetDefaults().GetPatroni()
	if patroni != nil && patroni.GetEnabled() {
		if err := validatePatroni(patroni, replicaCount); err != nil {
			return err
		}
		if etcdCount == 0 {
			return fmt.Errorf("patroni is enabled but no nodes have etcd service configured")
		}
	}

	return nil
}

func validateNode(node *database.Postgres_Node) error {
	if node == nil {
		return fmt.Errorf("node is nil")
	}
	if !nodeNameRegexp.MatchString(node.GetName()) {
		return fmt.Errorf("node name %q is invalid: must match ^[a-z][a-z0-9-]*$", node.GetName())
	}
	if !nodeHasAnyService(node) {
		return fmt.Errorf("node %q must have at least one service", node.GetName())
	}
	return nil
}

func nodeHasAnyService(node *database.Postgres_Node) bool {
	return node.GetPostgres() != nil ||
		node.GetEtcd() != nil ||
		node.GetPgbouncer() != nil ||
		node.GetBackup() != nil ||
		node.GetMonitoring() != nil
}

func validateSettings(s *database.Postgres_Settings) error {
	if s == nil {
		return fmt.Errorf("postgres settings is nil")
	}
	if s.GetStorageEngine() == database.Postgres_Settings_STORAGE_ENGINE_ORIOLEDB {
		v := s.GetVersion()
		if v != database.Postgres_Settings_VERSION_16 && v != database.Postgres_Settings_VERSION_17 {
			return fmt.Errorf("orioledb storage engine is only supported on postgres versions 16 and 17")
		}
	}
	return nil
}

func validatePatroni(patroni *database.Postgres_Settings_Patroni, replicaCount int) error {
	if patroni == nil || !patroni.GetEnabled() {
		return nil
	}
	if patroni.GetSynchronousMode() {
		requiredReplicas := int(patroni.GetSynchronousNodeCount())
		if requiredReplicas == 0 {
			requiredReplicas = 1
		}
		if replicaCount < requiredReplicas {
			return fmt.Errorf("synchronous_mode is enabled with %d sync nodes, but only %d replicas defined", requiredReplicas, replicaCount)
		}
	}
	return nil
}
