package spec

// ContractVersion identifies the server/UI/pipeline handoff, including diagnostic
// semantics and JSON presence rules. See live/tests/platform/contracts/HANDOFF.md.
// It follows the product while it is 0.x: a change to the contract bumps the
// minor (0.1.0 → 0.2.0) in the release that carries it; 1.0.0 is when a
// breaking change starts to need a new major and a migration/adapter.
const ContractVersion = "0.7.0"
