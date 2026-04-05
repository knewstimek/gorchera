# skip_leader 모드 설계 보고서

## 결론 요약

- **누적 실패 이력**: 직전 N개(N=3) 실패를 구조화된 `evaluator_failures` 배열로 주입. 전부 주입은 컨텍스트 폭발, 1개만은 회귀 유발.
- **히스토리 전달**: `buildCompactExecutorPayload`를 확장하여 `evaluator_failures []EvalFailure` 필드 추가. leader용 context_mode와 별개의 skip_leader 전용 경로.
- **놓치는 케이스**: `blocked` 상태 전환 메커니즘 필수. evaluator가 `"blocked"` 상태를 반환하면 즉시 job을 blocked 처리하여 감독관 개입 유도.
- **max_retries**: `max_steps`와 별도의 `max_eval_retries` 카운터(기본값 3). max_steps는 전체 executor 호출 횟수 상한으로 유지.
- **구현 방식**: 기존 leader 루프와 병렬로 존재하는 별도 함수 `runSkipLeaderLoop`. 기존 코드 변경 최소화.

---

## 1. 누적 실패 이력 깊이

### 권고안: 직전 3개 실패, 구조화된 배열

현재 `buildCompactExecutorPayload`는 직전 실패 1개만 `previous_failure` 필드로 주입한다 (protocol.go:729-742). skip_leader 모드에서는 evaluator 실패 -> executor 수정 -> evaluator 재검증 루프가 반복되므로, executor가 과거 실패 패턴을 인식하지 못하면 회귀가 발생한다.

**근거 -- 회귀 시나리오 분석:**

```
retry 1: "ParseConfig null check" -> executor 수정
retry 2: "ReadFile error handling" -> executor가 ParseConfig 영역을 건드려 회귀
retry 3: "ParseConfig null check again" -> 직전 1개만 보면 retry 1과 동일 정보
```

executor가 retry 1의 실패를 함께 보면 "이 영역은 이전에도 깨졌으니 주의"라는 맥락을 얻는다.

**왜 전부가 아닌 3개인가:**

- 기드라 포팅처럼 대규모 작업에서 retry가 5-10회 누적될 수 있다. 전체 이력은 executor 프롬프트를 비대하게 만든다.
- LLM의 실효적 주의(attention) 범위를 고려하면, 오래된 실패일수록 현재 코드 상태와 괴리가 커서 노이즈가 된다.
- 3개면 "현재 문제 + 직전 맥락 + 패턴 인식"이 가능하다.

**트레이드오프:**

| 깊이 | 장점 | 단점 |
|------|------|------|
| 1 (현재) | 프롬프트 최소 | 회귀 패턴 감지 불가 |
| 3 (권고) | 회귀 감지 + 프롬프트 적정 | 3회 이전 실패는 누락 |
| 전부 | 완전한 이력 | 프롬프트 폭발, 노이즈 |

**예외 처리**: 실패가 3회 미만이면 있는 만큼만 주입. 실패 reason이 200자 초과 시 truncate.

---

## 2. 히스토리 전달 방식

### 권고안: executor payload에 `evaluator_failures` 필드 추가

현재 구조를 기준으로 설계한다:

- leader용 `context_mode` (full/summary/minimal)는 `LeaderContextSummary` 필드를 통해 leader 프롬프트에 영향을 준다 (service.go:1395).
- executor용 `buildCompactExecutorPayload`는 context_mode와 무관하게 독립적으로 동작한다 (protocol.go:710).

skip_leader 모드에서는 leader가 없으므로 leader용 context_mode는 의미가 없다. 대신 executor payload를 확장한다.

**구체적 구조:**

```go
type skipLeaderPayload struct {
    JobID            string          `json:"job_id"`
    WorkspaceDir     string          `json:"workspace_dir"`
    WorkspaceMode    string          `json:"workspace_mode"`
    EvalRetryIndex   int             `json:"eval_retry_index"`
    MaxEvalRetries   int             `json:"max_eval_retries"`
    EvalFailures     []evalFailEntry `json:"evaluator_failures,omitempty"`
}

type evalFailEntry struct {
    RetryIndex int    `json:"retry_index"`
    Reason     string `json:"reason"`
    Status     string `json:"status"` // "failed" or "blocked"
}
```

**왜 별도 필드인가 (단순 텍스트 누적이 아닌 이유):**

1. 구조화된 데이터는 LLM이 파싱하기 쉽다. "retry 2에서 실패한 이유"를 명확히 구분할 수 있다.
2. 오케스트레이터가 기계적으로 truncate/필터링할 수 있다.
3. 기존 `previous_failure` 필드와 공존 가능 -- skip_leader가 아닌 일반 모드에서는 기존 동작 유지.

**구현 위치**: `buildCompactExecutorPayload` 함수를 수정하거나, skip_leader 전용 빌더 `buildSkipLeaderExecutorPayload`를 별도로 만든다. 후자가 기존 코드 영향 최소화 측면에서 권고.

**트레이드오프:**

- 별도 빌더: 코드 중복 소량 발생하나 기존 경로 안정성 보장
- 기존 빌더 확장: DRY하나 일반 모드에 의도치 않은 영향 가능성

---

## 3. 놓치는 케이스와 대응

### 3.1 의존 패키지 미포팅 (blocked 케이스)

**시나리오**: 패키지 A가 패키지 B를 import하는데, B가 아직 포팅되지 않았다. executor가 `go build`를 실행하면 컴파일 에러.

**현재 leader 루프에서의 처리**: leader가 "blocked" 액션을 반환하고, 감독관이 체인을 재구성하거나 의존 순서를 조정한다 (service.go:1417-1418).

**skip_leader 대응안:**

evaluator가 빌드 실패를 감지하고 reason에 "import cycle / dependency not available"을 명시하면, 오케스트레이터가 이를 기계적으로 판별할 수 있다. 구체적으로:

1. evaluator 또는 engine의 `automated_checks`가 빌드 실패를 잡는다 (이미 구현됨).
2. evaluator가 `status: "blocked"`를 반환하면 오케스트레이터는 retry하지 않고 즉시 job을 `blocked` 상태로 전환한다.
3. `blocked` 사유가 감독관에게 노출되어 체인 재구성 판단이 가능하다.

**핵심 규칙**: skip_leader 루프에서 evaluator `status`에 따른 분기:
- `"failed"`: retry 가능 -- executor에게 실패 이유 주입 후 재시도
- `"blocked"`: retry 불가 -- job을 blocked로 전환, 감독관 개입 대기

이 분기는 현재 leader 루프의 `mergeEvaluatorReport` 결과 처리와 일관된다 (service.go:1390-1401).

### 3.2 근본 설계 문제 (evaluator가 "fundamental design issue" 지적)

**시나리오**: evaluator가 "이 패키지의 구조 자체가 Go idiom에 맞지 않다"고 지적. executor만으로는 대응 불가.

**대응안:**

1. **retry 횟수 소진**: 동일 유형의 실패가 반복되면 자연스럽게 max_eval_retries에 도달하여 job이 실패한다. 감독관이 goal을 재설계한다.
2. **evaluator hint 활용**: evaluator reason에 "fundamental"/"design"/"architecture" 키워드가 포함되면 오케스트레이터가 즉시 blocked 처리하는 것도 가능하나, false positive 위험이 크다. **권고하지 않음.**
3. **실질적 대응**: 이 케이스는 leader가 있어도 대응이 어렵다. leader도 LLM이므로 근본 설계 문제를 해결하는 task_text를 생성할 역량이 제한적이다. 따라서 skip_leader 고유의 약점이 아니라 오케스트레이터 전반의 한계이다.

**결론**: max_eval_retries 소진 -> job 실패 -> 감독관 개입이 가장 현실적인 경로. 별도 키워드 감지 로직은 불필요한 복잡성.

### 3.3 executor가 task_text 없이 동작해야 하는 문제

leader 루프에서는 leader가 매 턴 `task_text`를 생성하여 executor에게 전달한다 (LeaderOutput.TaskText). skip_leader에서는 이 task_text를 누가 만드는가?

**대응안:**

- **초기 task_text**: job의 `goal` + `done_criteria`에서 기계적으로 생성. "다음을 구현하시오: {goal}. 완료 조건: {done_criteria}"
- **retry task_text**: evaluator 실패 이유를 포함한 템플릿. "이전 시도에서 다음 문제가 발견되었습니다: {reason}. 수정하시오."
- 이 템플릿은 오케스트레이터 코드에 하드코딩. LLM 호출 없이 문자열 조합.

---

## 4. max_retries 설계

### 권고안: `max_eval_retries` 별도 카운터 (기본값 3), `max_steps` 독립 유지

**현재 구조와의 관계:**

- `max_steps`는 leader 루프의 전체 반복 횟수 상한이다 (service.go:1258). 각 반복은 leader 호출 + worker 실행 1회.
- skip_leader 루프에서 "step"의 의미가 달라진다: leader 호출이 없으므로, 1 step = executor 호출 1회 + evaluator 검증 1회.

**왜 별도 카운터인가:**

1. `max_steps`를 공유하면 의미가 혼란스럽다. leader 모드에서 max_steps=10은 "leader 판단 10회"이고, skip_leader에서는 "executor retry 10회"가 된다. 같은 숫자가 전혀 다른 비용을 의미한다.
2. skip_leader는 단일 작업의 수렴을 목적으로 하므로 retry 상한이 낮아야 한다. 3-5회 retry로 수렴하지 않으면 근본 문제가 있을 가능성이 높다.
3. `max_steps`는 여전히 전체 executor 호출 횟수의 안전장치로 유지한다. `max_eval_retries`는 evaluator 실패 후 retry 횟수만 제한한다.

**구체적 동작:**

```
eval_retry_count = 0
loop:
    executor 실행 (max_steps 카운트 증가)
    evaluator 실행
    if passed: done
    if blocked: job blocked (즉시 종료)
    if failed:
        eval_retry_count++
        if eval_retry_count >= max_eval_retries: job failed
        else: evaluator 실패 이유 주입 후 continue loop
    if max_steps 도달: job blocked ("max_steps_exceeded")
```

**기본값 근거:**

- 3회: 첫 시도 + 2회 수정 기회. 경험적으로 LLM이 동일 문제를 3회 시도해서 못 고치면 4회차도 실패할 확률이 높다.
- Job 파라미터로 오버라이드 가능하게 설계: `max_eval_retries` 필드를 Job 구조체에 추가.

**트레이드오프:**

| 방식 | 장점 | 단점 |
|------|------|------|
| max_steps 공유 | 구현 단순 | 의미 혼란, 과도한 retry 허용 |
| max_eval_retries 별도 (권고) | 명확한 의미, 비용 제어 | 필드 추가 필요 |
| 하드코딩 3 | 가장 단순 | 유연성 없음 |

---

## 5. 전체 구현 권고

### 5.1 Job 구조체 변경

```go
// domain/types.go - Job struct에 추가
SkipLeader      bool `json:"skip_leader,omitempty"`
MaxEvalRetries  int  `json:"max_eval_retries,omitempty"` // 기본 3
```

`ChainGoal`에도 동일 필드 추가하여 체인에서 per-goal 설정 가능.

### 5.2 새 루프 함수

`service.go`에 `runSkipLeaderLoop` 함수를 추가한다. 기존 leader 루프(1258-1424행)와 병렬로 존재.

```
func (s *Service) runSkipLeaderLoop(ctx, job) (*Job, error):
    ensurePlanning (skip_planning과 조합 가능)
    task_text = buildInitialTaskText(job)
    for retry := 0; retry < maxEvalRetries; retry++:
        runWorkerStep(ctx, job, syntheticLeaderOutput(task_text))
        if job failed/blocked: return
        report = evaluateCompletion(ctx, job)
        if passed: done
        if blocked: return blocked
        task_text = buildRetryTaskText(job, report, recentFailures)
    return failed("max_eval_retries exceeded")
```

**핵심 설계 결정:**

1. `syntheticLeaderOutput` -- LeaderOutput 구조체를 기계적으로 생성. action="run_worker", target="executor", task_type="implement". 기존 `runWorkerStep`을 그대로 재사용.
2. `buildRetryTaskText` -- evaluator 실패 이유 + 최근 N개 실패 이력을 포함한 task_text 템플릿.
3. `buildSkipLeaderExecutorPayload` -- 기존 `buildCompactExecutorPayload` 확장판. `evaluator_failures` 배열 포함.

### 5.3 진입 분기

기존 `runLoop` (또는 해당 함수)에서 분기:

```go
if job.SkipLeader {
    return s.runSkipLeaderLoop(ctx, job)
}
// 기존 leader 루프
```

`ensurePlanning` 이후, leader 루프 진입 직전에 분기하는 것이 자연스럽다 (service.go:1252 직후).

### 5.4 skip_planning과의 조합

`skip_planning + skip_leader`는 가장 경량 모드가 된다:
- Planner LLM 호출 없음
- Leader LLM 호출 없음
- Executor + Evaluator만 동작
- 체인의 대량 반복 작업에 최적

이 조합이 기드라 포팅 같은 유스케이스의 핵심 시나리오이다.

### 5.5 관찰 가능성 (Observability)

skip_leader 루프에서도 기존 이벤트 시스템을 활용한다:

- `"skip_leader_retry"` -- evaluator 실패 후 retry 시작 시
- `"skip_leader_exhausted"` -- max_eval_retries 소진 시
- `gorchera_status`에서 `eval_retry_count` / `max_eval_retries` 노출

감독관이 `gorchera_steer`로 실행 중인 skip_leader job에 지시를 주입할 수 있어야 한다. 기존 `SupervisorDirective` 필드를 그대로 활용하되, retry task_text 생성 시 directive가 있으면 함께 주입.

### 5.6 구현 우선순위

1. **P0**: Job 필드 추가 + `runSkipLeaderLoop` 골격 + 분기 로직
2. **P0**: `buildSkipLeaderExecutorPayload` (evaluator_failures 포함)
3. **P1**: `buildRetryTaskText` 템플릿
4. **P1**: evaluator blocked/failed 분기 처리
5. **P2**: gorchera_status에 eval_retry 표시
6. **P2**: steer 연동
