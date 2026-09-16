// Preflight checks a compiled RunSpec without contacting Graphene or a cloud.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func main() {
	if err := check(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(in io.Reader, out io.Writer) error {
	var r spec.Run
	dec := json.NewDecoder(io.LimitReader(in, 32<<20))
	if err := dec.Decode(&r); err != nil {
		return fmt.Errorf("decode RunSpec: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("input must contain exactly one JSON object")
	}
	normalized, err := spec.NormalizeRun(r)
	if err != nil {
		return err
	}
	report := struct {
		OK       bool                 `json:"ok"`
		Estimate spec.DatasetEstimate `json:"dataset_estimate"`
		Issues   []spec.ResourceIssue `json:"issues"`
	}{true, spec.EstimateDataset(normalized.Workload.Segments), spec.CheckResources(normalized)}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
