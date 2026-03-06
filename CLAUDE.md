# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Stroppy Cloud is a distributed workflow orchestration system for automated database performance testing. It uses **Hatchet** as the task-scheduling backbone with a master/edge worker architecture: `master-worker` orchestrates test workflows while ephemeral `edge-worker` instances run on provisioned VMs to execute tasks (install software, run containers, execute tests).

All inter-service contracts are defined as **Protocol Buffers** (source in `tools/proto/`, generated Go in `internal/proto/`, generated TypeScript in `web/src/proto/`).

## Build & Development Commands

### Go Backend

```bash
make build                # Build all binaries to bin/
make run-master-worker    # Build + run master worker with zap-pretty
make run-test             # Build all + run test from examples/test.yaml
go test ./...             # Run all unit tests
go test ./internal/domain/provision/...  # Run tests for a specific package
go test -run TestName ./internal/...     # Run a single test
go test -tags integration ./...          # Run integration tests (need Docker)
```

### Infrastructure (Docker Compose)

```bash
make up-infra             # Start Hatchet + Valkey + Postgres + RabbitMQ
make down-infra           # Stop infrastructure
make clean-infra          # Stop + remove volumes
make up-dev               # Build and start master + edge workers in Docker
make down-dev             # Stop dev containers
```

### Web Frontend

```bash
cd web && yarn install
yarn dev                  # Vite dev server
yarn build                # TypeScript check + production build
yarn lint                 # ESLint
```

### Protobuf Generation

Proto sources are in `tools/proto/`. Generation is configured via `tools/proto/easyp.yaml` using easyp. Generated files go to `internal/proto/` (Go) and `web/src/proto/` (TypeScript). **Never edit generated proto files directly.**

## Architecture

### Binary Entry Points (`cmd/`)

- **`master-worker`** — Long-running daemon; registers workflows with Hatchet, orchestrates test lifecycles
- **`edge-worker`** — Ephemeral per-VM binary; accepts task kinds from env, executes containers/stroppy tasks
- **`run`** — One-shot CLI; parses YAML test spec, triggers workflow via Hatchet, waits for completion

### Internal Code Organization (`internal/`)

```
internal/
├── core/           # Shared utilities (logger, IDs, IPs, shutdown, hatchet-ext wrappers)
├── domain/         # Business logic
│   ├── workflows/  # Hatchet workflow DAG definitions
│   │   ├── test/   # Master-side: TestSuiteWorkflow, TestRunWorkflow (11-task DAG)
│   │   └── edge/   # Edge-side: stroppy install/run, container setup
│   ├── provision/  # Placement orchestration (ProvisionerService, lifecycle management)
│   ├── topology/   # Placement builders (Postgres, Picodata), validation, IP counting
│   ├── deployment/ # Cloud target backends via Registry (strategy pattern)
│   │   ├── yandex/ # Terraform + Yandex Cloud (embedded .tf files)
│   │   └── docker/ # Local Docker backend
│   ├── managers/   # Infrastructure managers (Valkey-backed network CIDR allocation)
│   └── edge/       # Edge worker helpers (container lifecycle, postgres docker-compose)
├── infrastructure/ # External service adapters (terraform executor, valkey client, S3)
└── proto/          # Generated protobuf code (DO NOT EDIT)
```

### Key Abstractions

- **`hatchet_ext.WTask[I,O]` / `PTask[PO,I,O]`** (`internal/core/hatchet-ext/task.go`) — Type-safe generic wrappers around Hatchet tasks that avoid `any` casts and auto-unmarshal parent outputs
- **`deployment.Registry`** (`internal/domain/deployment/deployment.go`) — Strategy pattern mapping `Target` enum to `Service` implementations; add new cloud providers by implementing the `Service` interface
- **`ProvisionerService`** (`internal/domain/provision/provision.go`) — Orchestrates full placement lifecycle: plan intent → build placement → deploy → destroy
- **`NetworkManager`** (`internal/domain/managers/network.go`) — Valkey-backed distributed CIDR reservation with locking

---

## bd (Beads) — Single Source of Truth

This project uses **bd (beads)** as the **sole issue tracker and task management system**. There is no plan.md. All work — tasks, decisions, architecture, progress — lives in bd issues.

Run `bd prime` for full workflow context. Run `bd hooks install` for auto-injection.
Do not sync bd in git (use `bd sync` separately).

### Quick Reference

```bash
bd ready                    # Find unblocked work (your "what's next")
bd show <id>                # Full issue details
bd create "Title" --type task --priority 2 --labels "domain,layer"
bd close <id>               # Complete work
bd update <id> --status in_progress
bd dep add <child> <parent> # Set dependency (child blocked by parent)
bd epic status              # Phase/epic completion overview
bd search "keyword"         # Find issues by text
bd sync                     # Sync with git (run at session end)
```

### Issue Hierarchy (mandatory structure)

All work is organized in a strict hierarchy via `--parent` and `bd dep add`:

```
Epic (phase/milestone)
  └── Feature (any new capability or meaningful change)
        └── Task (concrete step within a feature)
              └── Subtask (optional, small decomposition of a task)
```

**Rules:**
- Dependencies between issues use `bd dep add <blocked> <blocker>` — an issue cannot start until its blockers are closed
- `bd ready` shows only issues with ALL dependencies resolved

### Issue Types (mandatory `--type` flag)

| Type | When to use | Required fields |
|------|-------------|-----------------|
| `epic` | Phase or milestone grouping. Use `bd epic status` to track. | `--description` (expanded), `--set-labels`, `--notes` |
| `feature` | Any new capability or meaningful change. The primary work unit. | `--description` (expanded), `--acceptance`, `--set-labels`, `--notes` |
| `task` | A concrete step within a feature. Always `--parent <feature-id>`. | `--description`, `--design`, `--set-labels`, `--assignee` |
| `subtask` | Optional small step within a task. Always `--parent <task-id>`. | `--description`, `--design`, `--set-labels`, `--assignee` |
| `bug` | Broken or spec-deviating behavior. Always a blocker. | `--description` (expected vs actual), `--set-labels` |
| `chore` | Maintenance/cleanup: refactors, dead-code, linting, docs. | `--description`, `--set-labels` |

**Hierarchy:** `epic` → `feature` → `task` → `subtask` (optional)

### Required Fields by Type

| Field                     | epic | feature | task | subtask | bug | chore |
|---------------------------|------|---------|------|---------|-----|-------|
| `--description` (expanded) | **YES** | **YES** | short | short | expected vs actual | short |
| `--acceptance`            | — | **YES** | — | — | — | — |
| `--design`                | decisions | — | **YES** | **YES** | — | — |
| `--notes`                 | **YES** | **YES** | optional | optional | optional | optional |
| `--set-labels`            | **YES** | **YES** | **YES** | **YES** | **YES** | **YES** |
| `--assignee`              | — | — | **YES** | **YES** | optional | optional |
| `--parent`                | — | epic-id | feature-id | task-id | feature/task-id | epic-id |

### Field Descriptions

- **`--description`**: For `epic` and `feature` — expanded description explaining goals, scope, context, and motivation. For `task`/`subtask` — concise description of what to do.
- **`--acceptance`**: Required for `feature` only. Checklist of verifiable criteria that define "done".
- **`--design`**: Required for `task` and `subtask`. Technical approach: files to modify, libraries, algorithms, patterns. For `epic` — architectural decisions made by user.
- **`--notes`**: Free-form notes on any issue. Use for context, links, observations, progress updates.
- **`--set-labels`**: Semantic + scope tags on every issue (see Labels section below).
- **`--assignee`**: Agent role name. Required for `task` and `subtask`. Values: `researcher`, `implementer`, `tester`, `documenter`, `architect`.

### Agent Comments

Agents leave comments on `feature`, `task`, and `subtask` issues to record progress, blockers, and discoveries:

```bash
bd comment <id> "Discovered that library X requires explicit context" --author implementer
bd comment <id> "Tests pass. Coverage: 87%. No regressions." --author tester
bd comment <id> "Blocked: need user decision on retry policy" --author implementer
```

### Labels (mandatory on every issue)

Every issue MUST have labels describing **what it touches**. Use multiple labels. Labels show the semantic domain and scope of the item.

Determine available label categories from the project documentation. Typical categories:

| Category | Purpose |
|----------|---------|
| **Component** | Which part of the system this touches |
| **Domain** | Business or technical domain |
| **Layer** | `service-logic`, `infra`, `api`, `test`, `docs` |
| **Scope** | `breaking-change`, `refactor`, `bugfix`, `new-feature`, `optimization` |

### Hierarchy Rules

- Every `feature` MUST be a child of an `epic` (`--parent <epic-id>`).
- Every `task` MUST be a child of a `feature` (`--parent <feature-id>`).
- Every `subtask` MUST be a child of a `task` (`--parent <task-id>`).
- `bug` MUST reference the expected vs actual behavior in `--description`.
- `bug` is **always a blocker**: when created, immediately `bd dep add <blocked-feature-or-task> <bug-id>`.
- `bug` can be linked to a `feature` (`--parent <feature-id>`) or a `task` (`--parent <task-id>`) — wherever discovered.
- `chore` should never block a `feature` — if it does, promote it to `feature`.
- When in doubt between `feature` and `chore`, prefer `feature`.

### Issue Templates

**Epic** (expanded description + notes):

```bash
bd create "Phase N: <Component>" \
  --type epic \
  --priority 1 \
  --labels "<component>,<domain>,new-feature" \
  --description "$(cat <<'EOF'
**Scope**: <What this phase covers>.

**Context**: <Why this is needed, what depends on it>.

**Contains**: <High-level list of deliverables>.
EOF
)" \
  --notes "<References, links, prior decisions>."
```

**Feature** (expanded description + acceptance criteria):

```bash
bd create "<Feature title>" \
  --type feature \
  --priority 2 \
  --parent <epic-id> \
  --labels "<component>,<domain>,new-feature" \
  --description "$(cat <<'EOF'
**Goal**: <What this feature achieves>.

**Context**: <Current state and why change is needed>.

**Scope**: <Boundaries — what's in, what's out>.
EOF
)" \
  --acceptance "$(cat <<'EOF'
- [ ] <Verifiable criterion 1>
- [ ] <Verifiable criterion 2>
- [ ] Project compiles successfully
EOF
)" \
  --notes "<User decisions relevant to this feature>."
```

**Task** (design + assignee):

```bash
bd create "<Task title>" \
  --type task \
  --priority 2 \
  --parent <feature-id> \
  --assignee implementer \
  --labels "<component>,<domain>" \
  --description "<What to do>." \
  --design "$(cat <<'EOF'
**Files**:
- `<path/to/file>` (new|modify)

**Approach**:
1. <Step 1>
2. <Step 2>

**Verification**: <How to verify this works>
EOF
)"
```

**Bug** (always blocker, expected vs actual):

```bash
bd create "<Bug title>" \
  --type bug \
  --priority 1 \
  --parent <feature-id> \
  --labels "<domain>,bugfix" \
  --description "$(cat <<'EOF'
**Expected**: <What should happen>.
**Actual**: <What actually happens>.
**Cause** (suspected): <Root cause hypothesis>.
EOF
)"
# Bug is always a blocker:
bd dep add <feature-id> <bug-id>
```

**Architectural decisions** in epic's `--design` field:

```bash
bd update <epic-id> --design "$(cat <<'EOF'
**Decisions:**
- <Option A vs Option B> → user chose <decision>
- <Parameter> → <chosen value>
EOF
)"
```

### Every Action = Issue

Every repository action MUST be tracked in a bd issue:
- Implementation work → task under the relevant epic
- Bug discovered → `bd create --type bug` linked to the epic
- Refactor needed → `bd create --type chore` with dependency on the triggering task
- A task may contain multiple related actions (don't over-decompose into 1-line subtasks)

### Agents Assign Work via bd

Agents communicate work assignments through bd:
- Coordinator creates tasks and assigns them: `bd update <id> --assignee <role>`
- Agent claims work: `bd update <id> --claim`
- Agent marks progress: `bd update <id> --status in_progress`
- Agent completes: `bd close <id> --reason "done" --suggest-next`
- Agent discovers new work: `bd create "..." --parent <epic-id> --deps <current-task-id>`

---

### Workflow Execution Flow

1. CLI (`bin/run --file examples/test.yaml`) parses YAML into protobuf input
2. Hatchet dispatches to `TestSuiteWorkflow` → spawns N child `TestRunWorkflow` via `RunMany`
3. Each `TestRunWorkflow` is an 11-task DAG: validate → acquire network → plan placement → build placement → deploy → wait workers → setup containers → install stroppy → run stroppy → destroy
4. Edge workers poll Hatchet and execute assigned tasks on provisioned VMs

### State & Coordination

- **Workflow state**: Hatchet's Postgres
- **Network allocation**: Valkey sets keyed by `network:{target}:{name}` with distributed locking
- **Terraform workdirs**: `/tmp/stroppy-terraform/{deploymentId}/`

## Conventions

- Go module path: `github.com/stroppy-io/hatchet-workflow`
- Protobuf types are the canonical domain model — YAML configs are unmarshaled into proto types via `protoyaml`
- Task/workflow names use kebab-case string constants (e.g., `"stroppy-test-run"`, `"validate-input"`)
- Table-driven tests with `testify` assertions; integration tests use `//go:build integration` tag
- Frontend uses React 19 + TypeScript + Vite + Tailwind CSS + @xyflow/react for topology visualization



## Development Cycle

Every feature/task follows this cycle. Steps are sequential — each step's output feeds the next.

```
┌─────────────────────────────────────────────────────────┐
│                   COORDINATOR (opus)                     │
│  Owns the cycle. Delegates all work to sub-agents.       │
│  Asks user for EVERY architectural/behavioral decision.  │
└──┬───────┬───────┬───────┬───────┬───────┬───────┬──────┘
   │       │       │       │       │       │       │
   ▼       ▼       ▼       ▼       ▼       ▼       ▼
  1.RES  2.DOC  3.PLAN  4.ASK   5.IMPL  6.TEST  7.DOC
                  ▲        │
                  └────────┘ (loop until user approves plan)
```

### Step 1: Research (sonnet)

**Goal**: Gather all context needed for the task.

Coordinator spawns **parallel** research agents:

| Agent | Task |
|-------|------|
| **Codebase researcher** (sonnet) | Explore stroppy-cloud code relevant to the task. Read existing domain logic, proto, infrastructure. |
| **Doc searcher** (sonnet + context7) | Fetch documentation for libraries/tools involved. |

**Output**: Each agent returns a structured summary. Coordinator synthesizes into a task brief.

### Step 2: Documentation Search (sonnet + context7)

**Goal**: Find specific API docs, examples, and best practices.

Runs **in parallel with Step 1** when libraries are known upfront.

**Output**: API references, code examples, configuration patterns.

### Step 3: Plan (opus writes to docs/plan.md)

**Goal**: Design the solution and write a detailed plan.

Based on research results, Coordinator:

1. Identifies open questions that affect architecture/behavior
2. **Asks user ALL open questions** (Step 4 — may loop)
3. Designs the approach: files to create/modify, order of operations
4. **Writes the detailed plan into `docs/plan.md`** under the relevant phase
5. Each plan step includes: goal, files, dependencies, verification criteria

> This is the ONE step where opus does intellectual work directly.
> Plan is written to `docs/plan.md` BEFORE any code is written.

### Step 4: User Approval (interactive)

**Goal**: Resolve all ambiguity before writing code.

Coordinator MUST ask the user about:

| Category | Example questions |
|----------|-----------------|
| **Architecture** | "Should this be a new workflow task or extend an existing one?", "Should this live in provision/ or topology/?" |
| **API design** | "What fields should the new proto message contain?", "Should we add a new RPC or extend the existing one?" |
| **Behavior** | "Should failed edge tasks retry 3 or 5 times?", "What's the default timeout for Terraform apply?" |
| **Technology** | "Use Docker SDK directly or add testcontainers-go?", "Should we add a new Valkey key or extend the existing structure?" |
| **Scope** | "Should we implement this for all deployment targets or just Yandex?", "Is this needed for V1?" |
| **Data model** | "Should this be a new proto message or a field on an existing one?", "Node-centric or template-based?" |

**Rules**:
- Ask ALL questions at once (batch), not one by one
- Present options with trade-offs when possible
- Record all answers in `docs/plan.md` under "Questions resolved"
- If user's answer changes the plan — update plan before proceeding

**Loop**: Steps 3-4 repeat until the user approves the plan and all questions are resolved.

### Step 5: Implementation (sonnet)

**Goal**: Write code according to the plan from `docs/plan.md`.

Coordinator spawns **sequential** implementation agents (order matters):

```
Agent 1 (sonnet): "Write proto definition tools/proto/database/..."
Agent 2 (sonnet): "Run easyp to regenerate Go + TypeScript code"
Agent 3 (sonnet): "Implement domain logic internal/domain/..."
Agent 4 (sonnet): "Wire into workflow tasks internal/domain/workflows/..."
```

Each agent receives:
- The specific plan step from `docs/plan.md`
- Relevant research context (from Step 1)
- Relevant API docs (from Step 2)
- Reference code snippets (from Step 1)
- User decisions (from Step 4)

**Rules for implementers**:
- Follow existing patterns in `internal/domain/` (placement builders, workflow tasks, deployment services)
- Protobuf types are the canonical domain model — define data in proto first, then implement logic
- Use `hatchet_ext.WTask`/`PTask` wrappers for new workflow tasks
- Run `go build ./...` after each file change
- **If an implementer encounters an unexpected decision** → report back to Coordinator → Coordinator asks user

### Step 6: Testing & Verification (sonnet)

**Goal**: Verify the implementation compiles, works, and follows standards.

Coordinator spawns agents:

| Agent | Task |
|-------|------|
| **Compilation checker** (sonnet) | `go build ./...` — verify entire project compiles |
| **Test writer** (sonnet) | Write table-driven tests with `testify`; integration tests with `//go:build integration` tag |
| **Test runner** (sonnet) | Run `go test ./internal/domain/<package>/...` |
| **Verification reader** (sonnet) | Read modified files, check against existing patterns in the codebase, report issues |

**On failure**: Coordinator loops back to Step 5 with error context. Does NOT ask user unless the failure reveals an architectural problem.

**On success**: Coordinator proceeds to Step 7.

### Step 7: Documentation (haiku)

**Goal**: Update documentation to reflect changes.

Coordinator spawns documenter (haiku):

- Update `docs/plan.md`: mark `[x]` on completed tasks, add discovered subtasks
- Update `README.md` if architecture or services changed
- Add inline comments only where logic is non-obvious

**Rules for documenter**:
- Keep docs concise
- Do NOT add excessive comments to code
- Do NOT create new .md files unless explicitly requested

---