package compile

import "github.com/stroppy-io/stroppy-cloud/pipelines/spec"

func (c *compilation) prepareDataDisks() { spec.EnsureDiskPreparation(&c.out.Spec) }
