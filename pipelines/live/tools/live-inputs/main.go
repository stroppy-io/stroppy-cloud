// live-inputs compiles catalog topologies through the server's real compiler.
// It only writes inputs and an inventory; it never starts cloud resources.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type entry struct {
	Database string `json:"database"`
	Topology string `json:"topology"`
	Version  string `json:"version"`
	Script   string `json:"script"`
	Input    string `json:"input,omitempty"`
	Error    string `json:"error,omitempty"`
}

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	out := flag.String("output", "", "new output directory (required; never overwritten)")
	settingsFile := flag.String("provider", "../../tests/platform/quotas/quotas-yandex.json", "YC fixture with settings and credentials_secret")
	paramsFile := flag.String("database-params-file", "", "JSON object overriding database parameters (requires --database)")
	kind := flag.String("database", "", "one database kind, or all when empty")
	topologyID := flag.String("topology", "", "one topology, or all when empty")
	script := flag.String("script", "simple", "Stroppy workload")
	stroppyImage := flag.String("stroppy-image", "docker.stroppy.io/stroppy-io/stroppy:v6.0.0.62", "Stroppy image; use an immutable digest for development builds")
	duration := flag.Duration("duration", 30*time.Second, "measurement duration")
	allVersions := flag.Bool("all-versions", false, "compile every catalog version")
	baseline := flag.Bool("baseline", false, "enable both quick baseline tiers")
	mirror := flag.String("registry-mirror", "", "registry host proxying Docker Hub, GHCR and Quay (for example docker.stroppy.io)")
	flag.Parse()
	if *mirror != "" {
		u, err := url.Parse("https://" + *mirror)
		if err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return fmt.Errorf("--registry-mirror must be a registry host, optionally with a port")
		}
	}
	if *out == "" {
		return fmt.Errorf("--output is required")
	}
	raw, err := os.ReadFile(*settingsFile)
	if err != nil {
		return err
	}
	var provider spec.Quotas
	if err := json.Unmarshal(raw, &provider); err != nil {
		return err
	}
	if provider.Provider != spec.ProviderYandex {
		return fmt.Errorf("only YC inputs are allowed")
	}
	var overrides map[string]any
	if *paramsFile != "" {
		if *kind == "" {
			return fmt.Errorf("--database-params-file requires --database")
		}
		raw, err := os.ReadFile(*paramsFile)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &overrides); err != nil {
			return err
		}
		if overrides == nil {
			return fmt.Errorf("database parameter overrides must be an object")
		}
	}
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		return err
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	yc, _ := cat.Provider("yandex")
	if err := os.Mkdir(*out, 0o700); err != nil {
		return err
	}
	var inventory []entry
	for _, db := range cat.Databases {
		if *kind != "" && string(db.Kind) != *kind {
			continue
		}
		for _, version := range db.Versions {
			if !*allVersions && !version.Default {
				continue
			}
			for _, topo := range db.Topologies {
				if *topologyID != "" && topo.ID != *topologyID {
					continue
				}
				e := entry{Database: string(db.Kind), Topology: topo.ID, Version: version.Version, Script: *script}
				params := make(map[string]any, len(topo.Params))
				for k, v := range topo.Params {
					params[k] = v
				}
				if _, ok := params["version"]; ok {
					params["version"] = version.Version
				}
				if _, ok := params["image_tag"]; ok {
					params["image_tag"] = version.Version
				}
				for k, v := range overrides {
					params[k] = v
				}
				dparams, err := json.Marshal(params)
				if err != nil {
					return err
				}
				dspec, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: db.Kind, Version: version.Version, Params: dparams})
				if err != nil {
					e.Error = err.Error()
					inventory = append(inventory, e)
					continue
				}
				workload := map[string]any{"script": *script}
				if strings.HasPrefix(*script, "tpcb/") || strings.HasPrefix(*script, "tpcc/") {
					workload["scale_factor"] = 1
				}
				if *script == "tpch/tx" || *script == "tpcds" {
					workload["scale_factor"] = 0.01
				}
				if *script == "execute_sql" {
					workload["sql_body"] = "--= smoke\nSELECT 1;"
				}
				segment, err := json.Marshal(map[string]any{"name": "smoke", "workload": workload, "run": map[string]any{"executor": "constant-vus", "vus": 2, "duration": duration.String()}})
				if err != nil {
					return err
				}
				protocol := db.Protocols[0]
				if db.Kind == catalog.External {
					var connection struct {
						Protocol catalog.Protocol `json:"protocol"`
					}
					if err := json.Unmarshal(dspec.Params, &connection); err != nil {
						return err
					}
					protocol = connection.Protocol
				}
				wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: protocol, Segments: []json.RawMessage{segment}})
				if err != nil {
					e.Error = err.Error()
					inventory = append(inventory, e)
					continue
				}
				sizes := map[string]library.RoleSize{}
				for _, node := range derived.Plan.Nodes {
					sizes[node.Role] = library.RoleSize{Size: "S"}
				}
				compiled, err := compile.Compile(ctx, reg, compile.Input{
					RunID: uuid.New(), Tenant: "stroppy-live", Database: dspec, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs,
					Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: yc, ProviderKind: "yandex", ProviderSettings: provider.Settings,
					CredentialsSecret: provider.CredentialsSecret, ProviderConfigName: "t-stroppy-live", Catalog: cat,
					StroppyImage: *stroppyImage,
				})
				if err != nil {
					e.Error = err.Error()
					inventory = append(inventory, e)
					continue
				}
				if *baseline {
					compiled.Spec.Workload.Baseline = &spec.Baseline{Enabled: true, Quick: true}
				}
				for i := range compiled.Spec.Containers {
					compiled.Spec.Containers[i].Image = mirrorImage(compiled.Spec.Containers[i].Image, *mirror)
				}
				compiled.Spec.Workload.StroppyImage = mirrorImage(compiled.Spec.Workload.StroppyImage, *mirror)
				payload, err := json.MarshalIndent(compiled.Spec, "", "  ")
				if err != nil {
					return err
				}
				if _, err := reg.Bake(ctx, "spec.run@1", payload); err != nil {
					e.Error = err.Error()
					inventory = append(inventory, e)
					continue
				}
				e.Input = strings.NewReplacer("/", "-", ":", "-").Replace(e.Database+"-"+e.Topology+"-"+e.Version+"-"+e.Script) + ".json"
				if err := os.WriteFile(filepath.Join(*out, e.Input), append(payload, '\n'), 0o600); err != nil {
					return err
				}
				inventory = append(inventory, e)
			}
		}
	}
	if len(inventory) == 0 {
		return fmt.Errorf("no catalog entries matched")
	}
	raw, err = json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "inventory.json"), append(raw, '\n'), 0o600); err != nil {
		return err
	}
	ready := 0
	for _, e := range inventory {
		if e.Input != "" {
			ready++
		}
	}
	fmt.Printf("%d inputs compiled, %d entries need fixes; inventory: %s\n", ready, len(inventory)-ready, filepath.Join(*out, "inventory.json"))
	return nil
}
