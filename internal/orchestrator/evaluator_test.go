package orchestrator

import (
	"strings"
	"testing"

	"gorchera/internal/domain"
)

func TestVerificationSatisfiedNormalAcceptsBuildWithoutReview(t *testing.T) {
	t.Parallel()

	job := domain.Job{
		Steps: []domain.Step{
			{TaskType: "implement", Status: domain.StepStatusSucceeded},
			{TaskType: "build", Status: domain.StepStatusSucceeded},
		},
	}
	contract := VerificationContract{
		RequiredStepTypes: []string{"implement", "review", "test"},
	}

	passed, missing := verificationSatisfiedNormal(job, contract)
	if !passed {
		t.Fatalf("expected normal verification to pass, missing=%v", missing)
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing coverage, got %v", missing)
	}
}

func TestMergeEvaluatorReportNormalIgnoresOptionalProviderMissing(t *testing.T) {
	t.Parallel()

	job := domain.Job{
		Steps: []domain.Step{
			{Target: "B", TaskType: "implement", Status: domain.StepStatusSucceeded},
			{Target: "SYS", TaskType: "build", Status: domain.StepStatusSucceeded},
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
			{Target: "B", TaskType: "implement", Status: domain.StepStatusSucceeded},
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
			{Target: "B", TaskType: "implement", Status: domain.StepStatusSucceeded},
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
			{Axis: "test_coverage", Score: 0.5}, // below threshold 0.8
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
			{Target: "B", TaskType: "implement", Status: domain.StepStatusSucceeded},
		},
	}
	verification := VerificationContract{
		RequiredStepTypes: []string{"implement"},
		// no RubricAxes
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

func TestBuildSprintContractNormalRequiresImplementOnly(t *testing.T) {
	t.Parallel()

	contract := buildSprintContract(domain.Job{
		Goal:            "normal strictness contract",
		StrictnessLevel: "normal",
		DoneCriteria:    []string{"include system verification"},
	}, domain.PlanningArtifact{})

	if len(contract.RequiredStepTypes) != 1 || contract.RequiredStepTypes[0] != "implement" {
		t.Fatalf("expected only implement to be required, got %v", contract.RequiredStepTypes)
	}
	if contract.ThresholdSuccessCnt != 1 {
		t.Fatalf("expected success threshold 1, got %d", contract.ThresholdSuccessCnt)
	}
	if contract.ThresholdMinSteps != 1 {
		t.Fatalf("expected min steps threshold 1, got %d", contract.ThresholdMinSteps)
	}
}
