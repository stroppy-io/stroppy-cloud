package spec

// ContractVersion identifies the server/UI/pipeline handoff, including diagnostic
// semantics and JSON presence rules. See live/tests/platform/contracts/HANDOFF.md.
// It stays 0.0.0 until the first deploy: server, UI and pipelines ship from one
// commit, so the contract changes with the code and nothing is bumped. The
// first deploy freezes 1.0.0; from then a breaking change needs a new major and
// an explicit migration/adapter.
const ContractVersion = "0.0.0"
