package compile

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// customRecipe runs the tenant's image on every db machine: its env and
// command as given, its port on the host network, its data on the data disk
// when it names a directory, and its own ready check.
func customRecipe(c *compilation) error {
	p := c.params()
	image := c.in.Database.Image
	if image == "" {
		image = strParam(p, "image", "")
	}
	port := intParam(p, "port", pgPort)
	env := map[string]string{}
	if m, ok := p["env"].(map[string]any); ok {
		for k, v := range m {
			if s, ok := v.(string); ok {
				env[k] = s
			}
		}
	}
	var cmd []string
	if list, ok := p["cmd"].([]any); ok {
		for _, a := range list {
			if s, ok := a.(string); ok {
				cmd = append(cmd, s)
			}
		}
	}
	var check *spec.Healthcheck
	if hc := strParam(p, "healthcheck", ""); hc != "" {
		check = healthcheck("CMD-SHELL", hc)
	}
	dataDir := strParam(p, "data_dir", "")
	for _, m := range c.machinesOf(topology.RoleDB) {
		ct := spec.Container{
			Name: m + "-" + c.engineOf(topology.RoleDB), Role: topology.RoleDB, Machine: m, Image: image,
			Kind:        spec.ContainerKindDatabase,
			Env:         env,
			Cmd:         cmd,
			Ports:       []spec.Port{{Container: port, Host: port}},
			Healthcheck: check,
			Restart:     "always",
		}
		if dataDir != "" {
			ct.Mounts = []spec.Mount{{Source: dataMount + "/custom", Target: dataDir}}
		}
		c.add(ct)
	}
	return nil
}
