# Gorchera Principles

## Core Identity

Gorchera is not a conversational assistant. It is a workflow engine that persists state, routes work, enforces policy, and records evidence.

## Non-Negotiable Principles

### 1. Evaluator Gate Is Mandatory

- `complete` must always pass through `evaluateCompletion()`.
- A leader decision alone is never enough to mark a job `done`.
- Chain advancement after a goal also depends on the underlying job reaching evaluator-approved `done`.

### 2. Artifact-Based Handoff Only

- Do not pass full inter-agent conversation logs between agents.
- Handoffs are limited to structured JSON, summaries, status, and artifact references.
- Workers remain task-scoped and disposable.

### 3. Orchestrator-Owned Parallelism

- Executors do not spawn their own subordinate workers.
- Parallel fan-out is orchestrator-controlled only.
- Parallel work must stay within `max_parallel_workers = 2` and keep disjoint targets/write scopes.

### 4. Approval Rules Are Not Optional

- Safe workspace-local reads, search, build, test, lint, and approved local commands can proceed automatically.
- Network access, credential access, deploy, git push, workspace-external writes/commands, and mass delete require blocking.
- Do not encode approval bypasses in provider logic, prompts, or chain controls.

### 5. Harness Ownership Must Stay Enforced

- `/harness/*` is global operator inventory.
- `/jobs/{id}/harness/*` is job-scoped ownership surface.
- A job must not inspect or stop another job's owned process through the job-scoped surface.
- Ownership checks and inflight claims belong in the orchestrator service layer.

### 6. Cross-Platform Neutral Core

- Core orchestration remains ordinary Go code, not shell-specific control logic.
- Windows-only behavior must not leak into the core protocol or domain model.
- Runtime categories stay platform-neutral even if actual commands differ by OS.

### 7. No User-Turn Dependency

- The internal loop must keep progressing without waiting for an extra user turn.
- Agents should not ask follow-up questions in the normal loop.
- Missing information should surface as `blocked`, not as conversational prompting.

### 8. Supervisor Directive Principle

- Supervisor steering is an operator override for the next leader turn, not a free-form side channel.
- A supervisor directive has highest priority inside the next leader prompt, but only for that turn.
- The directive must be injected as a dedicated supervisor section and cleared after the leader call.
- Supervisor directives must not be propagated into worker-to-worker handoffs or baked into long-lived summaries as if they were ordinary job history.
- A supervisor directive cannot bypass evaluator gates, approval policy, harness ownership, chain terminal semantics, or scope restrictions.

## Protocol Rules

- All agent-facing contracts are structured JSON.
- Valid leader actions are:
  - `run_worker`
  - `run_workers`
  - `run_system`
  - `summarize`
  - `complete`
  - `fail`
  - `blocked`
- Valid worker statuses are:
  - `success`
  - `failed`
  - `blocked`

## Current Implementation Gaps To Remember

These are not principles; they are implementation facts that should not be mistaken for completed spec work.

| Area | Current implementation |
|------|-------------------------|
| Invalid leader JSON/schema | Immediate job failure, not re-request |
| Invalid worker JSON/schema | Immediate job failure, not re-request |
| Chain control surface | MCP only, not CLI/HTTP |
| Context compaction strategy | Prompt payload shaping only (`full`, `summary`, `minimal`) |

If one of these gaps is addressed in code later, update both this document and the architecture/status docs together.
