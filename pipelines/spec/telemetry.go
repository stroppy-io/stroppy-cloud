package spec

// Identities a run's telemetry is found by. Graphene stamps every log line
// and every scraped series with the run, namespace, agent and entity it
// came from (graphene pkg/obs: graphene.namespace, graphene.run,
// graphene.agent, graphene.entity); Stroppy's own exporter writes the run
// labels with underscores. A reader that knows the RunSpec turns a machine
// or a container into these values with the functions below — the same
// ones the pipeline names its agents and containers with.

// Telemetry attribute keys.
const (
	AttrNamespace = "graphene.namespace"
	AttrRun       = "graphene.run"
	AttrAgent     = "graphene.agent"
	AttrEntity    = "graphene.entity"
	// Native Stroppy series carry the run labels in Prometheus spelling.
	LabelNativeNamespace = "graphene_namespace"
	LabelNativeRun       = "graphene_run"
	LabelNativeSegment   = "stroppy_segment"
)

// AgentName is the agent of a machine. Unique per run inside the
// namespace: two concurrent runs may both have a "db-1".
func AgentName(runID, machine string) string {
	id := runID
	if len(id) > 8 {
		id = id[:8]
	}
	return id + "-" + machine
}

// ContainerName is the Docker record of a container. Container records
// share a Graphene namespace across all runs; the full run UUID separates
// identical logical names, including concurrent suite cells.
func ContainerName(runID, container string) string {
	return runID + "-" + container
}

// ContainerEntity is the telemetry entity of a container.
func ContainerEntity(runID, container string) string {
	return "docker/" + ContainerName(runID, container)
}
