package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

// probeRequest is the JSON body for POST /api/v1/probe.
type probeRequest struct {
	Script      string `json:"script"`                // e.g. "tpcc/procs", "tpcb/tx"
	DriverType  string `json:"driver_type,omitempty"` // e.g. "postgres", "mysql", "picodata"
	PoolSize    int    `json:"pool_size,omitempty"`
	ScaleFactor int    `json:"scale_factor,omitempty"`
}

// stroppyProbe handles POST /api/v1/probe.
// It builds a minimal stroppy-config.json, runs `stroppy probe -f <file> -o json`,
// and returns the probe output (env declarations, steps, sql sections, driver setups).
func (s *Server) stroppyProbe(w http.ResponseWriter, r *http.Request) {
	var req probeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.Script == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "script is required"})
		return
	}

	// Build a minimal RunConfig for probe.
	script := req.Script
	rc := &stroppypb.RunConfig{
		Version: "1",
		Script:  &script,
	}

	// Add driver if specified.
	if req.DriverType != "" {
		driverCfg := &stroppypb.DriverRunConfig{
			DriverType: req.DriverType,
			Url:        defaultDriverURL(req.DriverType),
		}
		if req.PoolSize > 0 {
			maxConns := int32(req.PoolSize)
			driverCfg.Pool = &stroppypb.DriverRunConfig_PoolConfig{
				MaxConns: &maxConns,
				MinConns: &maxConns,
			}
		}
		rc.Drivers = map[uint32]*stroppypb.DriverRunConfig{0: driverCfg}
	}

	// Add env overrides.
	if req.ScaleFactor > 0 {
		if rc.Env == nil {
			rc.Env = make(map[string]string)
		}
		rc.Env["SCALE_FACTOR"] = fmt.Sprintf("%d", req.ScaleFactor)
	}
	if req.PoolSize > 0 {
		if rc.Env == nil {
			rc.Env = make(map[string]string)
		}
		rc.Env["POOL_SIZE"] = fmt.Sprintf("%d", req.PoolSize)
	}

	// Serialize config to JSON.
	configBytes, err := protojson.MarshalOptions{
		UseProtoNames: false,
	}.Marshal(rc)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "marshal config: " + err.Error()})
		return
	}

	// Write temp file.
	tmpDir, err := os.MkdirTemp("", "stroppy-probe-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create temp dir: " + err.Error()})
		return
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "stroppy-config.json")
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write config: " + err.Error()})
		return
	}

	// Run stroppy probe.
	cmd := exec.CommandContext(r.Context(), "stroppy", "probe", "-f", configPath, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error":  "probe failed: " + err.Error(),
			"output": string(output),
		})
		return
	}

	// Return probe JSON output directly.
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

func defaultDriverURL(driverType string) string {
	switch driverType {
	case "postgres":
		return "postgres://postgres:postgres@localhost:5432"
	case "mysql":
		return "root@tcp(localhost:3306)/"
	case "picodata":
		return "postgres://admin:T0psecret@localhost:1331"
	default:
		return "localhost"
	}
}
