<!-- gorechera orchestrator -->
# Gorechera

Gorechera is a stateful multi-agent orchestration engine.

This repository currently contains a Go MVP that focuses on:

- structured leader and worker message schemas
- provider adapter boundaries
- role-based execution profiles for planner/reviewer/executor/tester/evaluator
- file-based job and artifact storage
- a bounded internal loop with retry-safe status transitions
- CLI and HTTP operator entry points
- read-only visibility for job state, events, and artifacts

The first implementation ships with a `mock` provider so the orchestration loop can run end-to-end before real Codex or Claude adapters are added.
Gorechera chooses provider/model/effort/tool-policy/fallback/budget per role instead of relying on one global provider setting.
The architecture is intended to remain cross-platform across Windows, macOS, and Linux.

See [docs/IMPLEMENTATION_STATUS.md](./docs/IMPLEMENTATION_STATUS.md) for the current MVP boundary, known bugs, and self-hosting readiness.

## Commands

```bash
go run ./cmd/gorechera run -goal "Create an orchestrator MVP"
go run ./cmd/gorechera run -goal "Create an orchestrator MVP" -profiles-file ./examples/role-profiles.sample.json
go run ./cmd/gorechera status -all
go run ./cmd/gorechera events -job <job-id>
go run ./cmd/gorechera artifacts -job <job-id>
go run ./cmd/gorechera verification -job <job-id>
go run ./cmd/gorechera resume -job <job-id>
go run ./cmd/gorechera approve -job <job-id>
go run ./cmd/gorechera retry -job <job-id>
go run ./cmd/gorechera cancel -job <job-id> -reason "operator pause"
go run ./cmd/gorechera reject -job <job-id> -reason "not approved"
go run ./cmd/gorechera harness-start -command go -category test -args "test,./internal/api,-run,TestHelperHarnessProcess,-count=1" -env "GO_WANT_HELPER_PROCESS=1"
go run ./cmd/gorechera harness-start -job <job-id> -command go -category test -args "test,./internal/api,-run,TestHelperHarnessProcess,-count=1" -env "GO_WANT_HELPER_PROCESS=1"
go run ./cmd/gorechera harness-list
go run ./cmd/gorechera harness-list -job <job-id>
go run ./cmd/gorechera harness-status -pid <pid>
go run ./cmd/gorechera harness-status -job <job-id> -pid <pid>
go run ./cmd/gorechera harness-stop -pid <pid>
go run ./cmd/gorechera harness-stop -job <job-id> -pid <pid>
go run ./cmd/gorechera harness-view -job <job-id>
go run ./cmd/gorechera stream -job <job-id> -server http://127.0.0.1:8080
go run ./cmd/gorechera serve -addr :8080
```

## HTTP API

The current MVP exposes read-first operator endpoints:

- `GET /healthz`
- `GET /jobs`
- `POST /jobs`
- `GET /jobs/{job-id}`
- `GET /jobs/{job-id}/events`
- `GET /jobs/{job-id}/events/stream`
- `GET /jobs/{job-id}/artifacts`
- `GET /jobs/{job-id}/verification`
- `GET /jobs/{job-id}/planning`
- `GET /jobs/{job-id}/evaluator`
- `GET /jobs/{job-id}/profile`
- `POST /jobs/{job-id}/resume`
- `POST /jobs/{job-id}/approve`
- `POST /jobs/{job-id}/retry`
- `POST /jobs/{job-id}/cancel`
- `POST /jobs/{job-id}/reject`
- `GET /jobs/{job-id}/harness`
- `GET /jobs/{job-id}/harness/processes`
- `POST /jobs/{job-id}/harness/processes`
- `GET /jobs/{job-id}/harness/processes/{pid}`
- `POST /jobs/{job-id}/harness/processes/{pid}/stop`
- `GET /harness/processes`
- `POST /harness/processes`
- `GET /harness/processes/{pid}`
- `POST /harness/processes/{pid}/stop`

The `/harness/*` routes expose the global runtime inventory, while `/jobs/{job-id}/harness/*` exposes only the processes owned by that job.
Use [examples/role-profiles.sample.json](./examples/role-profiles.sample.json) as the initial profile shape for per-role provider/model selection.
The current MVP persists role profiles on the job and uses them for leader/worker routing. Planner and evaluator are provider-backed phases now, so the role profile surface matches the runtime instead of only describing it.
The `verification` surface is read-only and combines the sprint contract, evaluator report, and role profiles into one inspection view so tester/evaluator intent is visible without re-reading the whole job state.
The `planning`, `verification`, and `profile` views also expose the same parallel worker policy so operators can see `max_parallel_workers = 2`, leader/orchestrator approval, and disjoint write scope requirements from the read surface.
Parallel worker fan-out is implemented in the orchestrator: `max_parallel_workers = 2`, disjoint write scopes are enforced, and worker context stays artifact-scoped and minimal.
`approve` consumes a pending approval request and replays the blocked system step with operator consent. `reject` records an operator rejection and clears the pending approval without resuming execution.
`cancel`, `approve`, `reject`, and `retry` are exposed as control-plane actions so operators can stop a job, approve or reject gated work, re-enter the loop, and stream events over SSE without touching the service core.
`harness-start`, `harness-list`, `harness-status`, `harness-stop`, and `harness-view` expose the runtime process manager directly so operators can inspect and control bounded local harness processes from CLI or HTTP. Passing `-job <job-id>` switches those commands to job-scoped ownership instead of the global runtime inventory.
