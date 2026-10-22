package compile

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// --- CockroachDB ------------------------------------------------------------
//
// doc: cockroachlabs.com/docs/stable/cockroach-start, cockroach-init,
// start-a-local-cluster-in-docker. One node = start-single-node; more =
// start --join=all + one init sidecar on the first node. Cluster settings
// from the "settings" template are applied by the same sidecar.

const (
	crdbPort     = 26257
	crdbHTTPPort = 8080
	crdbDataDir  = "/cockroach/cockroach-data"
)

func cockroachRecipe(c *compilation) error {
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	role := topology.RoleDB
	schemaID := c.schemaFor(role, "cfg.cockroach.flags@")
	nodes := c.machinesOf(role)
	join := make([]any, 0, len(nodes))
	for _, m := range nodes {
		join = append(join, fmt.Sprintf("%s:%d", ip(m), crdbPort))
	}
	for _, m := range nodes {
		overlay := map[string]any{
			"store_path": crdbDataDir, "listen_addr": fmt.Sprintf("0.0.0.0:%d", crdbPort), "http_addr": fmt.Sprintf("0.0.0.0:%d", crdbHTTPPort),
			"advertise_addr": fmt.Sprintf("%s:%d", ip(m), crdbPort), "insecure": true,
		}
		if len(nodes) > 1 {
			overlay["join"] = join
		}
		flags, err := c.render(role, schemaID, "conf", overlay)
		if err != nil {
			return err
		}
		cmd := []string{"start"}
		if len(nodes) == 1 {
			cmd = []string{"start-single-node"}
		}
		// The schema places --log last. Its YAML can contain spaces and
		// newlines and must remain one argv element; other flags are scalar.
		plain, logConfig, hasLog := strings.Cut(strings.TrimSpace(flags), " --log=")
		cmd = append(cmd, strings.Fields(plain)...)
		if hasLog {
			cmd = append(cmd, "--log="+logConfig)
		}
		c.add(spec.Container{
			Name: m + "-cockroach", Role: role, Machine: m, Image: image, Cmd: cmd,
			Kind:        spec.ContainerKindDatabase,
			Entrypoint:  []string{"/cockroach/cockroach"},
			Mounts:      []spec.Mount{{Source: dataMount + "/cockroach", Target: crdbDataDir}},
			Ports:       []spec.Port{{Container: crdbPort, Host: crdbPort}, {Container: crdbHTTPPort, Host: crdbHTTPPort}},
			Scrape:      "/_status/vars",
			ScrapePort:  crdbHTTPPort,
			Healthcheck: healthcheck("CMD-SHELL", fmt.Sprintf("curl -sf http://127.0.0.1:%d/health", crdbHTTPPort)),
			Restart:     "always",
		})
	}
	settings, err := c.render(role, schemaID, "settings", nil)
	if err != nil {
		return err
	}
	init := c.first(role)
	var script strings.Builder
	if len(nodes) > 1 {
		clusterName := strParam(c.effective(role, schemaID), "cluster_name", "stroppy")
		fmt.Fprintf(&script, "until cockroach init --insecure --cluster-name=%s --host=%s:%d || cockroach sql --insecure --host=%s:%d -e 'SELECT 1'; do sleep 3; done\n", "'"+strings.ReplaceAll(clusterName, "'", "'\"'\"'")+"'", ip(init), crdbPort, ip(init), crdbPort)
	}
	fmt.Fprintf(&script, "until cockroach sql --insecure --host=%s:%d -e 'SELECT 1'; do sleep 3; done\n", ip(init), crdbPort)
	if strings.TrimSpace(strings.Join(nonComment(settings), "\n")) != "" {
		fmt.Fprintf(&script, "cockroach sql --insecure --host=%s:%d -f %s/settings.sql\n", ip(init), crdbPort, confDir)
	}
	if extra := strParam(c.params(), "init_sql", ""); extra != "" {
		fmt.Fprintf(&script, "cockroach sql --insecure --host=%s:%d -f %s/init.sql\n", ip(init), crdbPort, confDir)
	}
	c.add(c.initSidecar(role, init, image, script.String(), []spec.File{
		{Path: confDir + "/settings.sql", Content: settings},
		{Path: confDir + "/init.sql", Content: strParam(c.params(), "init_sql", "")},
	}, c.dbContainers()))
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		return c.haproxy([]haproxyListener{{Name: "sql", BindPort: crdbPort, Servers: c.servers(role, crdbPort)}})
	}
	return nil
}

func nonComment(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "--") && !strings.HasPrefix(t, "#") {
			out = append(out, l)
		}
	}
	return out
}

// initSidecar runs a one-shot script inside the database image after the
// databases are up, then idles so the deploy sees it healthy.
func (c *compilation) initSidecar(role, machine, image, script string, files []spec.File, deps []string) spec.Container {
	return spec.Container{
		Name: machine + "-init", Role: role, Machine: machine, Image: image,
		Kind:        spec.ContainerKindAddon,
		Entrypoint:  []string{"/bin/sh"},
		Cmd:         []string{"-c", "set -e\n" + script + "touch /tmp/stroppy-init-done\nwhile true; do sleep 3600; done"},
		Files:       files,
		Healthcheck: &spec.Healthcheck{Cmd: []string{"CMD-SHELL", "test -f /tmp/stroppy-init-done"}, Interval: spec.Duration(10e9), Retries: 100},
		Restart:     "no",
		DependsOn:   deps,
	}
}
