# MESSAGE_SCHEMA_EXAMPLE.md

## Purpose
이 문서는 Gorchera 오케스트레이터와 리더/워커 에이전트가 주고받는 메시지 형식의 예시를 정의한다.
이 문서는 예시 문서이며, 실제 구현에서는 이 구조를 코드로 검증해야 한다.

## Scope

이 메시지 스키마는 provider-agnostic이다.
Codex와 Claude는 서로 다른 transport를 가질 수 있지만, 오케스트레이터 내부에서 주고받는 JSON 계약은 동일해야 한다.
CLI, HTTP API, 웹 UI는 이 계약 위에 얹히는 운영 인터페이스일 뿐이다.

## Rules

1. 모든 메시지는 JSON이어야 한다.
2. 자유 텍스트만 있는 응답은 허용하지 않는다.
3. 오케스트레이터는 모든 메시지를 파싱하고 필수 필드를 검증해야 한다.
4. 필수 필드가 없거나 허용되지 않은 값이면 invalid schema로 처리해야 한다.
5. invalid schema는 재시도 가능하지만 반복되면 실패 처리해야 한다.
6. worker는 질문하지 말고 success / failed / blocked 중 하나로 반환해야 한다.
7. leader는 run_worker / run_system / summarize / complete / fail / blocked 중 하나만 반환해야 한다.

## Common Conventions

### Agent Names
- A: leader agent
- B: implementation worker
- C: review worker
- D: test worker

### Artifact Reference
artifact는 path 또는 id 문자열로 전달한다.

예:
- "artifacts/project_summary.md"
- "artifacts/patch.diff"
- "artifact:test_report_001"

### Provider Notes
provider 이름이나 transport 세부사항은 이 메시지 계약에 포함하지 않는다.
그 정보는 SessionManager와 ProviderAdapter 계층에서 처리한다.

## Leader Output Schema

```json
{
  "action": "run_worker | run_system | summarize | complete | fail | blocked",
  "target": "B | C | D | none",
  "task_type": "implement | review | test | summarize | none",
  "task_text": "string",
  "artifacts": ["string"],
  "reason": "string",
  "next_hint": "string"
}
```

### Field Notes
- action: 필수
- target: run_worker일 때 필수
- task_type: run_worker일 때 필수
- task_text: run_worker일 때 필수
- artifacts: 선택
- reason: complete / fail / blocked일 때 필수
- next_hint: 선택
- system_action: run_system일 때 필수

## Worker Output Schema

```json
{
  "status": "success | failed | blocked",
  "summary": "string",
  "artifacts": ["string"],
  "blocked_reason": "string | null",
  "error_reason": "string | null",
  "next_recommended_action": "string | null"
}
```

### Field Notes
- status: 필수
- summary: 필수
- artifacts: 선택
- blocked_reason: status가 blocked일 때 필수
- error_reason: status가 failed일 때 필수
- next_recommended_action: 선택

## Example 1: Leader asks B to implement

```json
{
  "action": "run_worker",
  "target": "B",
  "task_type": "implement",
  "task_text": "Renderer init path를 분리하고 기존 public API는 유지해라.",
  "artifacts": [
    "artifacts/project_summary.md",
    "artifacts/current_notes.md"
  ],
  "reason": "",
  "next_hint": "구현 후 patch.diff와 구현 요약을 반환해라."
}
```

## Example 2: B returns success

```json
{
  "status": "success",
  "summary": "Renderer init path 분리 완료. public API 변경 없음.",
  "artifacts": [
    "artifacts/patch.diff",
    "artifacts/implementation_notes.md"
  ],
  "blocked_reason": null,
  "error_reason": null,
  "next_recommended_action": "review"
}
```

## Example 3: Leader asks the system harness to run a local command

```json
{
  "action": "run_system",
  "target": "SYS",
  "task_type": "build",
  "task_text": "Run the local Go test suite.",
  "artifacts": [
    "artifacts/sprint_contract.json"
  ],
  "reason": "",
  "next_hint": "Store the command result as an artifact and continue based on the exit code.",
  "system_action": {
    "type": "test",
    "command": "go",
    "args": ["test", "./..."],
    "workdir": "."
  }
}
```

## Example 4: Leader asks C to review

```json
{
  "action": "run_worker",
  "target": "C",
  "task_type": "review",
  "task_text": "patch.diff를 검토하고 lifetime 문제와 회귀 가능성을 점검해라.",
  "artifacts": [
    "artifacts/patch.diff",
    "artifacts/implementation_notes.md"
  ],
  "reason": "",
  "next_hint": "문제 목록과 승인 여부를 반환해라."
}
```

## Example 5: C returns blocked

```json
{
  "status": "blocked",
  "summary": "변경 의도는 이해되지만 메모리 소유권 정책이 명확하지 않다.",
  "artifacts": [
    "artifacts/review_report.json"
  ],
  "blocked_reason": "ownership policy is unclear",
  "error_reason": null,
  "next_recommended_action": "ask_leader_for_decision"
}
```

## Example 6: Leader decides to stop with blocked

```json
{
  "action": "blocked",
  "target": "none",
  "task_type": "none",
  "task_text": "",
  "artifacts": [
    "artifacts/review_report.json"
  ],
  "reason": "ownership policy is unclear and cannot be resolved automatically",
  "next_hint": ""
}
```

## Example 7: D returns test failure

```json
{
  "status": "failed",
  "summary": "renderer regression test 2개 실패",
  "artifacts": [
    "artifacts/test_report.json"
  ],
  "blocked_reason": null,
  "error_reason": "regression tests failed",
  "next_recommended_action": "return_to_implementation"
}
```

## Invalid Examples

### Invalid Leader Output: missing target

```json
{
  "action": "run_worker",
  "task_type": "implement",
  "task_text": "something"
}
```

이 메시지는 invalid다.
이유:
- run_worker인데 target이 없다.

### Invalid Worker Output: invalid status

```json
{
  "status": "waiting",
  "summary": "what should I do next?"
}
```

이 메시지는 invalid다.
이유:
- status는 success / failed / blocked 중 하나여야 한다.
- 질문형 응답은 허용되지 않는다.

### Invalid Worker Output: blocked without blocked_reason

```json
{
  "status": "blocked",
  "summary": "cannot continue",
  "artifacts": []
}
```

이 메시지는 invalid다.
이유:
- blocked면 blocked_reason이 필요하다.

## Orchestrator Error Handling

오케스트레이터는 아래 규칙으로 처리한다.

### Case 1: JSON parse failure
처리:
- invalid_message_schema 기록
- 같은 agent에 1회 재요청
- 반복되면 failed 처리

### Case 2: required field missing
처리:
- missing_fields 기록
- 같은 agent에 스키마 오류와 함께 재요청
- 반복되면 failed 처리

### Case 3: unsupported action
처리:
- action 거부
- invalid_action 기록
- leader에 재출력 요청
- 반복되면 failed 처리

### Case 4: approval policy violation
처리:
- action 실행 금지
- blocked 또는 failed 처리
- 필요 시 leader에 정책 위반 상태 전달

## Example Error Response from Orchestrator

```json
{
  "error": "invalid_message_schema",
  "missing_fields": ["target", "task_type"],
  "retryable": true
}
```

## Recommended Validation Order

1. JSON parse
2. message type detection
3. required field check
4. enum value check
5. policy check
6. execution

## Implementation Note

이 문서는 예시 문서다.
실제 구현에서는 아래를 코드로 강제해야 한다.

- JSON schema validation
- enum validation
- approval policy validation
- retry limit
- invalid schema repetition detection
