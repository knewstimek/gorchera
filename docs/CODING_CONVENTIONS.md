# Gorchera Coding Conventions

## Build And Test

```bash
go build ./...
go test ./...
```

## Go Style

- Use `gofmt` output without local deviation.
- Return errors early; avoid unnecessary nesting.
- Keep domain JSON fields in `snake_case`.
- Prefer small, explicit helpers over hidden cross-package magic.
- Comments should explain a non-obvious rule, not restate the code.

## Domain Type Rules

- All cross-package domain types live in `internal/domain/types.go`.
- Add new job, step, chain, profile, or contract fields there first.
- Keep package-internal transport or helper structs local to their package.
- If a new status is added, also add the corresponding validation helper in `types.go`.

## State Persistence Pattern

For any job or chain mutation:

```go
entity.Field = value
s.addEvent(job, "event_kind", "message") // job only
s.touch(job)                             // or s.touchChain(chain)
if err := s.state.SaveJob(ctx, job); err != nil {
    return nil, err
}
```

Guidelines:
- Persist before starting asynchronous follow-up work.
- Do not update chain state only in memory and then launch the next goal.
- When a mutation changes terminal semantics, update both the in-memory struct and the persisted record before returning.

## Provider Adapter Rules

Interfaces:
- `Adapter`: `RunLeader`, `RunWorker`
- `PlannerRunner`: `RunPlanner`
- `EvaluatorRunner`: `RunEvaluator`

Registration:
- Register adapters in `provider.NewRegistry()`.

Selection:
- Provider resolution is role-specific.
- Use `SessionManager.resolveProfile()` / `resolveAdapter()` instead of re-implementing fallback logic.
- `fallback_provider` is resolved in `adapterForProfile()`.

Model handling:
- Claude consumes the selected model directly.
- Codex should only emit `--model` for Codex/GPT-family values.
- Do not silently treat Claude shorthand model names as valid Codex model flags.

Error handling:
- New provider transport/classification work belongs in `internal/provider/errors.go` and `internal/provider/command.go`.
- When adding a new provider error kind, also decide its `RecommendedAction`.

## Prompt And Schema Rules

- Update prompt builders in `internal/provider/protocol.go`.
- Update schema validation in `internal/schema/validate.go`.
- Every new leader action or worker status needs both:
  - schema validation
  - orchestrator handling in `runLoop()` or worker execution paths

Leader context:
- `ContextMode` must stay normalized to `full`, `summary`, or `minimal`.
- Supervisor directives must remain a separate prompt section, not duplicated inside serialized job payloads.

## Verification Contract Rules

- Planning generates the persisted verification contract.
- Test tasks should be decorated with verification-contract context through `decorateTaskForVerification()`.
- Do not bypass evaluator gating by writing `done` directly.
- If you change completion semantics, update:
  - `planning.go`
  - `verification.go`
  - `evaluator.go`
  - docs

## Artifact Rules

- Keep artifact writes atomic through `ArtifactStore`.
- Worker artifacts should prefer `FileContents` when available.
- System artifacts should store the full runtime result JSON.
- `Step.DiffSummary` is reserved for workspace diff visibility; do not overload it with arbitrary notes.

## Runtime And Approval Rules

- Add new system task types in `mapSystemTask()` and in runtime/policy allowlists together.
- Approval policy is category- and scope-based. Do not special-case risky behavior in a provider adapter.
- Workspace-relative command directories must continue to flow through `resolveSystemWorkdir()`.

## Chain Extension Guide

When extending the chain system, make changes in this order.

1. Domain model:
   - Add statuses or fields in `internal/domain/types.go`.
   - Update `ValidChainStatus` / `ValidChainGoalStatus` as needed.

2. Persistence:
   - Confirm the new fields round-trip through `StateStore` JSON save/load.
   - Add store tests if the extension changes persisted semantics.

3. Orchestrator lifecycle:
   - Update `StartChain`, `startChainGoal`, `advanceChain`, `handleChainCompletion`, `handleChainTerminalState`, and any operator control methods that the new behavior affects.
   - Preserve the invariant that only orchestrator-owned code starts the next chain goal.
   - Preserve evaluator-gated completion. A chain goal is not `done` until the underlying job is evaluator-approved `done`.

4. Control-plane surface:
   - Add MCP tools in `internal/mcp/server.go` only if the behavior is intentionally exposed.
   - Do not document or expose chain operations through CLI or HTTP unless they are actually wired.
   - If wait semantics are needed, follow the existing MCP polling pattern instead of inventing a second status mechanism.

5. Cancellation/pausing semantics:
   - Pausing must stop advancement, not force-kill the active goal.
   - Cancelling or skipping an active goal must go through `interruptChainGoalJob()` so the job state is persisted consistently.
   - New terminal chain statuses must short-circuit `advanceChain()`.

6. Tests:
   - Add service tests for lifecycle behavior.
   - Add MCP tests if a new MCP tool or response path is added.
   - Add domain/status validation tests when new statuses are introduced.

7. Documentation:
   - Update `docs/ARCHITECTURE.md` for lifecycle semantics and control surfaces.
   - Update `docs/IMPLEMENTATION_STATUS.md` for newly available behavior.
   - Update `docs/PRINCIPLES.md` if the change affects non-bypassable invariants.

## Test Style

- Prefer focused table-driven tests for validation and routing.
- Use end-to-end mock-provider tests for orchestrator loop behavior.
- When changing provider routing, cover leader, planner, evaluator, executor, reviewer, and tester paths explicitly.
- When changing chain behavior, cover both happy-path advancement and terminal interruption cases.

## Documentation Rules

When code changes:
- Update architecture docs for lifecycle, control surface, or package-boundary changes.
- Update implementation-status docs for newly implemented or still-missing behavior.
- Update principles docs when a new invariant or non-bypassable operator rule is introduced.
- Do not document spec-only behavior as implemented unless the current code path exists.
