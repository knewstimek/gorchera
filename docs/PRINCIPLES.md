# Gorechera Principles

## Core Identity

Gorechera는 AI가 아니라 일반 프로그램이다.
상태 관리자이자 라우터이며, 메시지를 이해하려 하지 않는다.

## 깨면 안 되는 원칙

### 1. Evaluator Gate 필수
- `complete` action은 반드시 `evaluateCompletion()`을 거침
- evaluator가 passed를 반환해야만 done 상태 전이 가능
- gate를 우회하는 경로를 만들지 않음

### 2. Artifact 기반 Handoff
- agent 간 전체 대화 로그 전달 금지
- 전달 단위: 구조화된 JSON, artifact path, summary, 상태값
- worker는 task-scoped session으로 실행 후 폐기

### 3. Orchestrator 주도 병렬 실행
- executor가 자체적으로 하위 worker를 spawn하지 않음
- 병렬 fan-out은 orchestrator가 정책에 따라 집행
- max_parallel_workers = 2, disjoint write scope 필수

### 4. Approval Policy
- 안전한 작업(읽기, 빌드, 테스트, 검색)은 자동 진행
- 위험한 작업(네트워크, 배포, git push, credential, 대량 삭제)은 blocked
- 에이전트는 질문 대신 blocked 상태를 반환

### 5. Harness Ownership 분리
- `/harness/*` = 전역 runtime inventory (운영자 진단용)
- `/jobs/{id}/harness/*` = job-scoped ownership surface
- job-scoped surface에서 다른 job 소유 pid 접근 거부
- ownership check는 service 레벨에서 강제

### 6. Cross-Platform 중립
- 코어 오케스트레이터는 OS 중립 Go 코드
- Windows Terminal은 선택적 디버그 뷰일 뿐
- bash/PowerShell 전용 가정을 프로토콜 수준에 넣지 않음
- runtime 명령 카테고리는 공통, 실제 명령 조합은 OS별로 다를 수 있음

### 7. No User Turn Dependency
- 내부 루프는 사용자 입력 없이 계속 진행
- 에이전트는 질문하지 않음 (blocked 반환)
- "다음 뭐할까요?" 같은 질의 금지

## Protocol Rules

- 모든 에이전트 응답은 구조화된 JSON
- Leader actions: run_worker, run_workers, run_system, summarize, complete, fail, blocked
- Worker statuses: success, failed, blocked

### 스펙 vs 현재 구현 차이 (주의)

| 스펙 | 현재 구현 |
|------|-----------|
| JSON 파싱 실패 시 재요청 | 즉시 failJob (service.go:373) |
| 필수 필드 누락 시 재요청 | 즉시 failJob (service.go:376) |
| invalid schema 반복 시 실패 | 첫 실패에서 바로 fail |

스펙대로 재요청 로직을 추가하려면 runLoop 안에서 retry 카운터를 두어야 함.
