package orchestrator

import (
	"testing"

	"gorechera/internal/domain"
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
