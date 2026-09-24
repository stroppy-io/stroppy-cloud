module github.com/stroppy-io/stroppy-cloud/live-tools

go 1.26.5

require (
	github.com/google/uuid v1.6.0
	github.com/stroppy-io/stroppy-cloud v0.0.0
	github.com/stroppy-io/stroppy-cloud/pipelines v0.0.0
)

require (
	cel.dev/expr v0.25.2 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cbroglie/mustache v1.4.0 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-faster/jx v1.2.0 // indirect
	github.com/google/cel-go v0.30.0 // indirect
	github.com/gopherex/schemapb/go v0.0.0-20260923103231-06c47e842fa3 // indirect
	github.com/gopherex/xlog v1.0.2 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/stroppy-io/stroppy-cloud => ../../../..

replace github.com/stroppy-io/stroppy-cloud/pipelines => ../../..

replace github.com/hashicorp/terraform-provider-aws => github.com/upbound/terraform-provider-aws v0.0.0-20260305123303-f7691456b787
