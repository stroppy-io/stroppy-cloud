package topology

import (
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
)

// RequiredIPCount calculates the number of IP addresses (and VMs) needed
// to deploy the database described by the given template.
// In the node-centric model this is simply the number of declared nodes.
func RequiredIPCount(tmpl *database.Database_Template) int {
	if tmpl == nil {
		return 0
	}

	switch t := tmpl.GetTemplate().(type) {
	case *database.Database_Template_PostgresInstance:
		if t.PostgresInstance == nil {
			return 0
		}
		return 1

	case *database.Database_Template_PostgresCluster:
		cluster := t.PostgresCluster
		if cluster == nil {
			return 0
		}
		return len(cluster.GetNodes())

	case *database.Database_Template_PicodataInstance:
		if t.PicodataInstance == nil {
			return 0
		}
		return 1

	case *database.Database_Template_PicodataCluster:
		cluster := t.PicodataCluster
		if cluster == nil {
			return 0
		}
		return len(cluster.GetNodes())
	default:
		return 0
	}
}
