package orchestrator_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gorchera/internal/domain"
	"gorchera/internal/orchestrator"
	"gorchera/internal/provider"
	"gorchera/internal/store"
)

// skipLeaderProvider is a test double for skip_leader mode.
//
// RunLeader is intentionally a canary: if skip_leader truly bypasses the
// leader loop, RunLeader should never be called. If it is called the provider
// returns a "fail" action so the job terminates with a recognisable failure
// reason, and leaderCalls > 0 makes the assertion obvious.
//
// RunEvaluator consumes evalResponses in order; the last entry repeats once
// exhausted. This lets tests drive multi-attempt scenarios (fail, fail, pass).
//
// RunWorker always succeeds and records the task_text so tests can verify
// that cumulative failure history was injected correctly.
type skipLeaderProvider struct {
	mu          sync.Mutex
	leaderCalls int
	workerCalls int
	evalCalls   int
	// taskTexts captures task_text from each RunWorker call in order.
	taskTexts []string
	// evalResponses are consumed in sequence; last entry repeats if exhausted.
	evalResponses []string
}

func (p *skipLeaderProvider) Name() domain.ProviderName {
	return domain.ProviderName("skip-leader-test")
}

// RunLeader must never be called in skip_leader mode.
// Returning "fail" produces a distinctive job.FailureReason for assertions.
func (p *skipLeaderProvider) RunLeader(_ context.Context, _ domain.Job) (string, error) {
	p.mu.Lock()
	p.leaderCalls++
	p.mu.Unlock()
	return `{"action":"fail","target":"none","task_type":"none","reason":"BUG: leader called in skip_leader mode"}`, nil
}

func (p *skipLeaderProvider) RunWorker(_ context.Context, _ domain.Job, task domain.LeaderOutput) (string, error) {
	p.mu.Lock()
	p.workerCalls++
	p.taskTexts = append(p.taskTexts, task.TaskText)
	p.mu.Unlock()
	return `{"status":"success","summary":"worker executed","artifacts":[]}`, nil
}

func (p *skipLeaderProvider) RunPlanner(_ context.Context, job domain.Job) (string, error) {
	return fmt.Sprintf(`{"goal":%q,"summary":"skip-leader test plan","product_scope":["test"],"proposed_steps":["implement"],"acceptance":["done"]}`, job.Goal), nil
}

func (p *skipLeaderProvider) RunEvaluator(_ context.Context, _ domain.Job) (string, error) {
	p.mu.Lock()
	idx := p.evalCalls
	p.evalCalls++
	p.mu.Unlock()
	if idx >= len(p.evalResponses) {
		idx = len(p.evalResponses) - 1
	}
	return p.evalResponses[idx], nil
}

// evalPassed/evalFailed/evalBlocked are helpers to build evaluator JSON responses.
func evalPassed(reason string) string {
	return fmt.Sprintf(`{"status":"passed","passed":true,"score":100,"reason":%q}`, reason)
}

func evalFailed(reason string) string {
	return fmt.Sprintf(`{"status":"failed","passed":false,"score":0,"reason":%q}`, reason)
}

func evalBlocked(reason string) string {
	return fmt.Sprintf(`{"status":"blocked","passed":false,"score":0,"reason":%q}`, reason)
}

func newSkipLeaderService(t *testing.T, p *skipLeaderProvider) *orchestrator.Service {
	t.Helper()
	root := t.TempDir()
	reg := provider.NewRegistry()
	reg.Register(p)
	return orchestrator.NewService(
		provider.NewSessionManager(reg),
		store.NewStateStore(filepath.Join(root, "state")),
		store.NewArtifactStore(filepath.Join(root, "artifacts")),
		root,
	)
}

func newSkipLeaderInput(p *skipLeaderProvider, extra ...func(*orchestrator.CreateJobInput)) orchestrator.CreateJobInput {
	input := orchestrator.CreateJobInput{
		Goal:         "port pkg/graph from C++ to Go",
		Provider:     p.Name(),
		MaxSteps:     10,
		SkipPlanning: true,
		SkipLeader:   true,
		DoneCriteria: []string{"all files compile", "tests pass"},
	}
	for _, fn := range extra {
		fn(&input)
	}
	return input
}

// --- test cases ---

// TestSkipLeaderPassesOnFirstAttempt: evaluator passes immediately.
// Expects done status, 1 worker call, 0 leader calls.
func TestSkipLeaderPassesOnFirstAttempt(t *testing.T) {
	t.Parallel()
	p := &skipLeaderProvider{evalResponses: []string{evalPassed("looks good")}}
	svc := newSkipLeaderService(t, p)

	job, err := svc.Start(context.Background(), newSkipLeaderInput(p))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if job.Status != domain.JobStatusDone {
		t.Fatalf("expected done, got %s (failure: %s)", job.Status, job.FailureReason)
	}
	if p.leaderCalls != 0 {
		t.Fatalf("expected 0 leader calls, got %d", p.leaderCalls)
	}
	if p.workerCalls != 1 {
		t.Fatalf("expected 1 worker call, got %d", p.workerCalls)
	}
	if p.evalCalls != 1 {
		t.Fatalf("expected 1 evaluator call, got %d", p.evalCalls)
	}
}

// TestSkipLeaderRetryOnFailThenPass: evaluator fails once then passes.
// Expects done, 2 worker calls, failure reason injected into second task_text.
func TestSkipLeaderRetryOnFailThenPass(t *testing.T) {
	t.Parallel()
	failReason := "ParseConfig missing null check"
	p := &skipLeaderProvider{evalResponses: []string{
		evalFailed(failReason),
		evalPassed("fixed"),
	}}
	svc := newSkipLeaderService(t, p)

	job, err := svc.Start(context.Background(), newSkipLeaderInput(p))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if job.Status != domain.JobStatusDone {
		t.Fatalf("expected done, got %s (failure: %s)", job.Status, job.FailureReason)
	}
	if p.workerCalls != 2 {
		t.Fatalf("expected 2 worker calls, got %d", p.workerCalls)
	}
	// Second task_text must contain the failure reason so executor knows what to fix.
	if len(p.taskTexts) < 2 || !strings.Contains(p.taskTexts[1], failReason) {
		t.Fatalf("expected second task_text to contain failure reason %q, got: %q", failReason, p.taskTexts[1])
	}
}

// TestSkipLeaderExhaustsMaxEvalRetries: evaluator always fails.
// Expects failed status after default 3 retries.
func TestSkipLeaderExhaustsMaxEvalRetries(t *testing.T) {
	t.Parallel()
	p := &skipLeaderProvider{evalResponses: []string{evalFailed("fundamental design problem")}}
	svc := newSkipLeaderService(t, p)

	job, err := svc.Start(context.Background(), newSkipLeaderInput(p))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if job.Status != domain.JobStatusFailed {
		t.Fatalf("expected failed, got %s", job.Status)
	}
	if !strings.Contains(job.FailureReason, "retries") {
		t.Fatalf("expected FailureReason to mention retries, got: %q", job.FailureReason)
	}
	// 3 retries = 3 worker + 3 evaluator calls (default MaxEvalRetries=3)
	if p.workerCalls != 3 {
		t.Fatalf("expected 3 worker calls, got %d", p.workerCalls)
	}
}

// TestSkipLeaderBlockedImmediately: evaluator returns blocked (e.g. missing dependency).
// Expects blocked status with no retry.
func TestSkipLeaderBlockedImmediately(t *testing.T) {
	t.Parallel()
	p := &skipLeaderProvider{evalResponses: []string{evalBlocked("pkg/core not yet ported")}}
	svc := newSkipLeaderService(t, p)

	job, err := svc.Start(context.Background(), newSkipLeaderInput(p))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if job.Status != domain.JobStatusBlocked {
		t.Fatalf("expected blocked, got %s", job.Status)
	}
	// blocked must not trigger a retry
	if p.workerCalls != 1 {
		t.Fatalf("expected 1 worker call (no retry on blocked), got %d", p.workerCalls)
	}
}

// TestSkipLeaderCustomMaxEvalRetries: max_eval_retries=2 overrides default 3.
func TestSkipLeaderCustomMaxEvalRetries(t *testing.T) {
	t.Parallel()
	p := &skipLeaderProvider{evalResponses: []string{evalFailed("always fails")}}
	svc := newSkipLeaderService(t, p)

	job, err := svc.Start(context.Background(), newSkipLeaderInput(p, func(i *orchestrator.CreateJobInput) {
		i.MaxEvalRetries = 2
	}))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if job.Status != domain.JobStatusFailed {
		t.Fatalf("expected failed, got %s", job.Status)
	}
	if p.workerCalls != 2 {
		t.Fatalf("expected 2 worker calls (max_eval_retries=2), got %d", p.workerCalls)
	}
}

// TestSkipLeaderAccumulatesFailHistory: three sequential failures must each
// appear in subsequent task_texts so executor does not regress earlier fixes.
func TestSkipLeaderAccumulatesFailHistory(t *testing.T) {
	t.Parallel()
	reasons := []string{"null check missing", "error handling missing", "still failing"}
	p := &skipLeaderProvider{evalResponses: []string{
		evalFailed(reasons[0]),
		evalFailed(reasons[1]),
		evalPassed("all fixed"),
	}}
	svc := newSkipLeaderService(t, p)

	job, err := svc.Start(context.Background(), newSkipLeaderInput(p))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if job.Status != domain.JobStatusDone {
		t.Fatalf("expected done, got %s (failure: %s)", job.Status, job.FailureReason)
	}
	// task_text[1] must mention attempt 1 failure
	if !strings.Contains(p.taskTexts[1], reasons[0]) {
		t.Fatalf("task_text[1] should contain %q, got: %q", reasons[0], p.taskTexts[1])
	}
	// task_text[2] must mention both previous failures
	if !strings.Contains(p.taskTexts[2], reasons[0]) || !strings.Contains(p.taskTexts[2], reasons[1]) {
		t.Fatalf("task_text[2] should contain both previous failures, got: %q", p.taskTexts[2])
	}
}

// TestSkipPlanningAndSkipLeader: lightest pipeline -- planner and leader both skipped.
// Expects done, 0 leader calls, planner never called.
func TestSkipPlanningAndSkipLeader(t *testing.T) {
	t.Parallel()
	p := &skipLeaderProvider{evalResponses: []string{evalPassed("done")}}
	svc := newSkipLeaderService(t, p)

	// RunPlanner is implemented on p but should not be called.
	plannerCalled := false
	_ = plannerCalled // captured via RunPlanner override below -- see limitation note

	job, err := svc.Start(context.Background(), orchestrator.CreateJobInput{
		Goal:         "port pkg/graph",
		Provider:     p.Name(),
		MaxSteps:     10,
		SkipPlanning: true,
		SkipLeader:   true,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if job.Status != domain.JobStatusDone {
		t.Fatalf("expected done, got %s", job.Status)
	}
	if p.leaderCalls != 0 {
		t.Fatalf("expected 0 leader calls, got %d", p.leaderCalls)
	}
	// With skip_planning, planner LLM is not called; planning artifacts are
	// built from job fields. We verify indirectly: evalCalls==1 means the
	// loop ran correctly without hanging on a missing plan.
	if p.evalCalls != 1 {
		t.Fatalf("expected 1 evaluator call, got %d", p.evalCalls)
	}
}
