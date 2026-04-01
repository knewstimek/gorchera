# Gorchera

Go stateful multi-agent orchestration engine with self-improvement capabilities.

**Gorchera** (Go + Orchestra) coordinates AI agents (GPT, Claude) to plan, implement, review, test, and evaluate software tasks autonomously. A supervisor agent (e.g., Claude Opus via MCP) monitors and steers the workflow.

## Features

- **6-role pipeline**: planner -> leader -> executor/reviewer/tester -> evaluator
- **3 strictness levels**: strict (implement+review+test), normal (implement only), lenient (results only)
- **3 context modes**: full, summary, minimal -- controls leader prompt payload size
- **Job chaining**: sequential multi-goal execution with automatic advancement
- **Supervisor steering**: mid-flight directive injection via `gorechera_steer`
- **Provider adapters**: GPT/Codex, Claude, Mock -- with per-role model selection
- **Self-improvement**: Gorchera can modify its own codebase via orchestrated jobs
- **Error classification**: 12 error types with 3-strike retry policy
- **Token tracking**: rough per-job and per-step token/cost estimation
- **Security**: SUPERVISOR injection prevention, workspace validation, steer authorization
- **MCP server**: 13 tools for supervisor agent integration (stdio JSON-RPC 2.0)

## Quick Start

```bash
go build ./...
go run ./cmd/gorechera mcp          # Start MCP server for Claude Code integration
go run ./cmd/gorechera run -goal "Add a hello function" -provider codex
go run ./cmd/gorechera status -all
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `gorechera_start_job` | Start a single job |
| `gorechera_start_chain` | Start sequential job chain |
| `gorechera_status` | Get job status |
| `gorechera_chain_status` | Get chain status |
| `gorechera_steer` | Inject supervisor directive |
| `gorechera_events` | Get job events |
| `gorechera_artifacts` | Get job artifacts |
| `gorechera_approve` | Approve blocked action |
| `gorechera_reject` | Reject blocked action |
| `gorechera_retry` | Retry failed job |
| `gorechera_cancel` | Cancel running job |
| `gorechera_resume` | Resume blocked job |
| `gorechera_list_jobs` | List all jobs |

## Architecture

See [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) for package structure, state machine, and core loop.

## Documentation

1. [ARCHITECTURE.md](./docs/ARCHITECTURE.md) -- package structure, state machine, core loop
2. [IMPLEMENTATION_STATUS.md](./docs/IMPLEMENTATION_STATUS.md) -- current state, resolved issues
3. [PRINCIPLES.md](./docs/PRINCIPLES.md) -- inviolable design principles
4. [CODING_CONVENTIONS.md](./docs/CODING_CONVENTIONS.md) -- coding rules, extension guides
5. [ORCHESTRATOR_SPEC_UPDATED.md](./docs/ORCHESTRATOR_SPEC_UPDATED.md) -- detailed design spec

## License

MIT
