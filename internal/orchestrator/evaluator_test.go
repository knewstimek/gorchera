package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorchera/internal/domain"
)

func TestVerificationSatisfiedNormalRequiresEngineEvidence(t *testing.T) {
	t.Parallel()

	// A step with no artifacts at all has no engine_build/engine_test paths,
	// so it is treated as a legacy job and engine verification is skipped.
	// Verification should pass (legacy compat -- see C1 fix).
	job := domain.Job{
		Steps: []domain.Step{
			{TaskType: "implement", Status: domain.StepStatusSucceeded},
		},
	}
	contract := VerificationContract{
		RequiredStepTypes: []string{"implement"},
	}

	passed, missing := verificationSatisfiedNormal(job, contract)
	if !passed {
		t.Fatalf("expected legacy job (no engine artifacts) to pass verification, missing=%v", missing)
	}
}

func TestVerificationSatisfiedNormalRequiresEngineEvidenceWhenPathPresent(t *testing.T) {
	t.Parallel()

	// A step that has an engine_build artifact path but the file does not exist
	// (or is unreadable) must NOT be treated as a legacy job -- the path signals
	// that engine verification was attempted. Verification should fail.
	job := domain.Job{
		Steps: []domain.Step{
			{
				TaskType: "implement",
				Status:   domain.StepStatusSucceeded,
				// Non-existent path, but the filename contains "engine_build"
				// so loadEngineCheckArtifacts will try (and fail) to read it.
				Artifacts: []string{"/nonexistent/engine_build_0.json"},
			},
		},
	}
	contract := VerificationContract{
		RequiredStepTypes: []string{"implement"},
	}

	passed, missing := verificationSatisfiedNormal(job, contract)
	if passed {
		t.Fatalf("expected verification to fail when engine artifact path is present but unreadable, missing=%v", missing)
	}
	if len(missing) == 0 {
		t.Fatal("expected missing engine verification coverage")
	}
}

func TestVerificationSatisfiedNormalAcceptsSkippedEngineChecks(t *testing.T) {
	t.Parallel()

	artifacts := writeEngineArtifactsForTest(t, engineCheckSkipped, engineCheckSkipped)
	job := domain.Job{
		Steps: []domain.Step{
			{TaskType: "implement", Status: domain.StepStatusSucceeded, Artifacts: artifacts},
		},
	}

	passed, missing := verificationSatisfiedNormal(job, VerificationContract{RequiredStepTypes: []string{"implement"}})
	if !passed {
		t.Fatalf("expected skipped engine checks to satisfy coverage, missing=%v", missing)
	}
}

func TestMergeEvaluatorReportNormalIgnoresOptionalProviderMissing(t *testing.T) {
	t.Parallel()

	job := domain.Job{
		Steps: []domain.Step{
			{Target: "B", TaskType: "implement", Status: domain.StepStatusSucceeded, Artifacts: writeEngineArtifactsForTest(t, engineCheckPassed, engineCheckPassed)},
		},
	}
	verification := VerificationContract{
		RequiredStepTypes: []string{"implement"},
	}
	sprint := domain.SprintContract{
		RequiredStepTypes:   []string{"implement"},
		ThresholdSuccessCnt: 1,
		ThresholdMinSteps:   1,
		StrictnessLevel:     "normal",
	}
	providerReport := domain.EvaluatorReport{
		Status:           "blocked",
		Passed:           false,
		Score:            67,
		Reason:           "missing required step coverage: review, test",
		MissingStepTypes: []string{"review", "test"},
	}

	report := mergeEvaluatorReport(job, verification, sprint, providerReport)
	if !report.Passed {
		t.Fatalf("expected merged report to pass, got %#v", report)
	}
	if report.Status != "passed" {
		t.Fatalf("expected passed status, got %q", report.Status)
	}
	if report.Score != 100 {
		t.Fatalf("expected score 100, got %d", report.Score)
	}
	if len(report.MissingStepTypes) != 0 {
		t.Fatalf("expected optional provider missing types to be ignored, got %v", report.MissingStepTypes)
	}
}

func TestMergeEvaluatorReportRubricAllPass(t *testing.T) {
	t.Parallel()

	job := domain.Job{
		Steps: []domain.Step{
			{Target: "B", TaskType: "implement", Status: domain.StepStatusSucceeded, Artifacts: writeEngineArtifactsForTest(t, engineCheckPassed, engineCheckPassed)},
		},
	}
	verification := VerificationContract{
		RequiredStepTypes: []string{"implement"},
		RubricAxes: []domain.RubricAxis{
			{Name: "functionality", Weight: 0.6, MinThreshold: 0.7},
			{Name: "code_quality", Weight: 0.4, MinThreshold: 0.6},
		},
	}
	sprint := domain.SprintContract{
		RequiredStepTypes:   []string{"implement"},
		ThresholdSuccessCnt: 1,
		StrictnessLevel:     "normal",
	}
	providerReport := domain.EvaluatorReport{
		Status: "passed",
		Passed: true,
		Score:  90,
		Reason: "all steps succeeded",
		RubricScores: []domain.RubricScore{
			{Axis: "functionality", Score: 0.9},
			{Axis: "code_quality", Score: 0.8},
		},
	}

	report := mergeEvaluatorReport(job, verification, sprint, providerReport)
	if !report.Passed {
		t.Fatalf("expected rubric all-pass report to pass, got %#v", report)
	}
	if report.Status != "passed" {
		t.Fatalf("expected status passed, got %q", report.Status)
	}
	if len(report.RubricScores) != 2 {
		t.Fatalf("expected 2 rubric scores stored, got %d", len(report.RubricScores))
	}
	for _, rs := range report.RubricScores {
		if !rs.Passed {
			t.Fatalf("expected axis %q to pass, got score %.2f", rs.Axis, rs.Score)
		}
	}
}

func TestMergeEvaluatorReportRubricAxisFail(t *testing.T) {
	t.Parallel()

	job := domain.Job{
		Steps: []domain.Step{
			{Target: "B", TaskType: "implement", Status: domain.StepStatusSucceeded, Artifacts: writeEngineArtifactsForTest(t, engineCheckPassed, engineCheckPassed)},
		},
	}
	verification := VerificationContract{
		RequiredStepTypes: []string{"implement"},
		RubricAxes: []domain.RubricAxis{
			{Name: "functionality", Weight: 0.6, MinThreshold: 0.7},
			{Name: "test_coverage", Weight: 0.4, MinThreshold: 0.8},
		},
	}
	sprint := domain.SprintContract{
		RequiredStepTypes:   []string{"implement"},
		ThresholdSuccessCnt: 1,
		StrictnessLevel:     "normal",
	}
	providerReport := domain.EvaluatorReport{
		Status: "passed",
		Passed: true,
		Score:  85,
		Reason: "implement succeeded",
		RubricScores: []domain.RubricScore{
			{Axis: "functionality", Score: 0.9},
			{Axis: "test_coverage", Score: 0.5},
		},
	}

	report := mergeEvaluatorReport(job, verification, sprint, providerReport)
	if report.Passed {
		t.Fatalf("expected rubric axis fail to demote report, got passed=true")
	}
	if report.Status != "failed" {
		t.Fatalf("expected status failed, got %q", report.Status)
	}
	if len(report.RubricScores) != 2 {
		t.Fatalf("expected 2 rubric scores stored, got %d", len(report.RubricScores))
	}
	failCount := 0
	for _, rs := range report.RubricScores {
		if !rs.Passed {
			failCount++
		}
	}
	if failCount != 1 {
		t.Fatalf("expected exactly 1 failed axis, got %d", failCount)
	}
	if !strings.Contains(report.Reason, "test_coverage") {
		t.Fatalf("expected reason to mention failed axis, got %q", report.Reason)
	}
}

func TestMergeEvaluatorReportNoRubric(t *testing.T) {
	t.Parallel()

	job := domain.Job{
		Steps: []domain.Step{
			{Target: "B", TaskType: "implement", Status: domain.StepStatusSucceeded, Artifacts: writeEngineArtifactsForTest(t, engineCheckPassed, engineCheckPassed)},
		},
	}
	verification := VerificationContract{
		RequiredStepTypes: []string{"implement"},
	}
	sprint := domain.SprintContract{
		RequiredStepTypes:   []string{"implement"},
		ThresholdSuccessCnt: 1,
		StrictnessLevel:     "normal",
	}
	providerReport := domain.EvaluatorReport{
		Status: "passed",
		Passed: true,
		Score:  90,
		Reason: "implement succeeded",
	}

	report := mergeEvaluatorReport(job, verification, sprint, providerReport)
	if !report.Passed {
		t.Fatalf("expected no-rubric report to pass unchanged, got %#v", report)
	}
	if report.Status != "passed" {
		t.Fatalf("expected status passed, got %q", report.Status)
	}
	if len(report.RubricScores) != 0 {
		t.Fatalf("expected no rubric scores when no axes defined, got %v", report.RubricScores)
	}
}

func TestBuildSprintContractBalancedRequiresReview(t *testing.T) {
	t.Parallel()

	contract := buildSprintContract(domain.Job{
		Goal:         "balanced pipeline contract",
		PipelineMode: string(domain.PipelineModeBalanced),
	}, domain.PlanningArtifact{})

	if len(contract.RequiredStepTypes) != 2 || contract.RequiredStepTypes[0] != "implement" || contract.RequiredStepTypes[1] != "review" {
		t.Fatalf("expected implement and review to be required, got %v", contract.RequiredStepTypes)
	}
	if contract.ThresholdSuccessCnt != 2 {
		t.Fatalf("expected success threshold 2, got %d", contract.ThresholdSuccessCnt)
	}
}

func TestBuildSprintContractLightRequiresImplementOnly(t *testing.T) {
	t.Parallel()

	contract := buildSprintContract(domain.Job{
		Goal:         "light pipeline contract",
		PipelineMode: string(domain.PipelineModeLight),
	}, domain.PlanningArtifact{})

	if len(contract.RequiredStepTypes) != 1 || contract.RequiredStepTypes[0] != "implement" {
		t.Fatalf("expected only implement to be required, got %v", contract.RequiredStepTypes)
	}
	if contract.ThresholdSuccessCnt != 1 {
		t.Fatalf("expected success threshold 1, got %d", contract.ThresholdSuccessCnt)
	}
}

func writeEngineArtifactsForTest(t *testing.T, buildStatus, testStatus string) []string {
	t.Helper()

	dir := t.TempDir()
	artifacts := []struct {
		name   string
		record EngineCheckArtifact
	}{
		{
			name: "step-01-engine_build.json",
			record: EngineCheckArtifact{
				Kind:    "build",
				Status:  buildStatus,
				Command: "go build ./...",
				Reason:  "test fixture",
			},
		},
		{
			name: "step-01-engine_test.json",
			record: EngineCheckArtifact{
				Kind:    "test",
				Status:  testStatus,
				Command: "go test ./...",
				Reason:  "test fixture",
			},
		},
	}
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		path := filepath.Join(dir, artifact.name)
		data, err := json.Marshal(artifact.record)
		if err != nil {
			t.Fatalf("failed to marshal engine artifact: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("failed to write engine artifact: %v", err)
		}
		paths = append(paths, path)
	}
	return paths
}
