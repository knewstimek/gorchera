package schema

import (
	"testing"

	"gorechera/internal/domain"
)

func TestValidateLeaderOutputRunWorkerRequiresFields(t *testing.T) {
	err := ValidateLeaderOutput(domain.LeaderOutput{
		Action:   "run_worker",
		Target:   "none",
		TaskType: "implement",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateLeaderOutputRunWorkersSuccess(t *testing.T) {
	err := ValidateLeaderOutput(domain.LeaderOutput{
		Action: "run_workers",
		Tasks: []domain.WorkerTask{
			{
				Target:   "B",
				TaskType: "implement",
				TaskText: "build the core",
			},
			{
				Target:   "C",
				TaskType: "review",
				TaskText: "review the core",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateLeaderOutputRunWorkersRequiresDistinctTargets(t *testing.T) {
	err := ValidateLeaderOutput(domain.LeaderOutput{
		Action: "run_workers",
		Tasks: []domain.WorkerTask{
			{
				Target:   "B",
				TaskType: "implement",
				TaskText: "build the core",
			},
			{
				Target:   "B",
				TaskType: "review",
				TaskText: "review the core",
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateLeaderOutputCompleteRequiresReason(t *testing.T) {
	err := ValidateLeaderOutput(domain.LeaderOutput{
		Action:   "complete",
		Target:   "none",
		TaskType: "none",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateLeaderOutputRunSystemRequiresSystemAction(t *testing.T) {
	err := ValidateLeaderOutput(domain.LeaderOutput{
		Action:   "run_system",
		Target:   "SYS",
		TaskType: "build",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateLeaderOutputRunSystemSuccess(t *testing.T) {
	err := ValidateLeaderOutput(domain.LeaderOutput{
		Action:   "run_system",
		Target:   "SYS",
		TaskType: "build",
		SystemAction: &domain.SystemAction{
			Type:    domain.SystemActionBuild,
			Command: "go",
			Args:    []string{"test", "./..."},
		},
	})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateWorkerOutputBlockedRequiresReason(t *testing.T) {
	err := ValidateWorkerOutput(domain.WorkerOutput{
		Status:  "blocked",
		Summary: "cannot continue",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateWorkerOutputSuccess(t *testing.T) {
	err := ValidateWorkerOutput(domain.WorkerOutput{
		Status:  "success",
		Summary: "done",
	})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateVerificationContractRequiresChecks(t *testing.T) {
	err := ValidateVerificationContract(domain.VerificationContract{
		Version: 1,
		Goal:    "verify",
		Scope:   []string{"implementation"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateVerificationContractSuccess(t *testing.T) {
	err := ValidateVerificationContract(domain.VerificationContract{
		Version:          1,
		Goal:             "verify",
		Scope:            []string{"implementation"},
		RequiredCommands: []string{"go test ./..."},
		RequiredChecks:   []string{"done gate"},
	})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateVerificationReportSuccess(t *testing.T) {
	err := ValidateVerificationReport(domain.VerificationReport{
		Status:   "passed",
		Passed:   true,
		Reason:   "ok",
		Evidence: []string{"go test ./..."},
	})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
