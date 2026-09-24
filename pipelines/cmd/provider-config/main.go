package main

import (
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/providerconfig"
)

func main() {
	pipeline.Main(providerconfig.PipelineID, providerconfig.Run, pipeline.WithConcurrency(pipeline.Parallel))
}
