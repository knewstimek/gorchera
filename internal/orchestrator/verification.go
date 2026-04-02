package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorchera/internal/domain"
)

type VerificationContract struct {
	Version              int                `json:"version"`
	Goal                 string             `json:"goal"`
	Summary              string             `json:"summary"`
	SprintContractRef    string             `json:"sprint_contract_ref,omitempty"`
	PlanningArtifactRefs []string           `json:"planning_artifact_refs,omitempty"`
	RequiredStepTypes    []string           `json:"required_step_types,omitempty"`
	AcceptanceCriteria   []string           `json:"acceptance_criteria,omitempty"`
	TesterInstructions   []string           `json:"tester_instructions,omitempty"`
	EvaluatorCriteria    []string           `json:"evaluator_criteria,omitempty"`
	// RubricAxes mirrors domain.VerificationContract.RubricAxes when loaded
	// from a persisted contract JSON. Used by mergeEvaluatorReport to enforce
	// per-axis score thresholds returned by the evaluator provider.
	RubricAxes           []domain.RubricAxis `json:"rubric_axes,omitempty"`
}

func buildVerificationContract(job domain.Job, planning domain.PlanningArtifact, sprint domain.SprintContract, planningArtifactRefs []string) VerificationContract {
	testerInstructions := []string{
		"Read the verification contract before running tests.",
		"Use the contract to choose only the tests needed for the current step.",
		"Return the exact commands, outcomes, and artifacts produced.",
	}
	evaluatorCriteria := []string{
		"tester output references the verification contract",
		"tester output includes concrete artifacts or logs",
		"required step coverage matches the sprint contract",
	}
	return VerificationContract{
		Version:              1,
		Goal:                 firstNonEmptyValue(&planning, func(p *domain.PlanningArtifact) string { return p.Goal }, job.Goal),
		Summary:              verificationSummary(job, planning, sprint),
		SprintContractRef:    job.SprintContractRef,
		PlanningArtifactRefs: append([]string(nil), planningArtifactRefs...),
		RequiredStepTypes:    append([]string(nil), sprint.RequiredStepTypes...),
		AcceptanceCriteria:   append([]string(nil), sprint.AcceptanceCriteria...),
		TesterInstructions:   testerInstructions,
		EvaluatorCriteria:    evaluatorCriteria,
	}
}

func verificationSummary(job domain.Job, planning domain.PlanningArtifact, sprint domain.SprintContract) string {
	required := strings.Join(sprint.RequiredStepTypes, ", ")
	if required == "" {
		required = "implement, review, test"
	}
	return fmt.Sprintf("Verify %s with required steps: %s", firstNonEmptyValue(&planning, func(p *domain.PlanningArtifact) string { return p.Summary }, job.Goal), required)
}

func buildPersistedVerificationContract(job domain.Job, planning domain.PlanningArtifact, sprint domain.SprintContract, contract VerificationContract, contractPath string) *domain.VerificationContract {
	if planning.VerificationContract != nil {
		cloned := *planning.VerificationContract
		cloned.Scope = append([]string(nil), cloned.Scope...)
		cloned.RequiredCommands = append([]string(nil), cloned.RequiredCommands...)
		cloned.RequiredArtifacts = append([]string(nil), cloned.RequiredArtifacts...)
		cloned.RequiredChecks = append([]string(nil), cloned.RequiredChecks...)
		cloned.DisallowedActions = append([]string(nil), cloned.DisallowedActions...)
		if len(cloned.RequiredArtifacts) == 0 {
			cloned.RequiredArtifacts = append(cloned.RequiredArtifacts, append([]string(nil), contract.PlanningArtifactRefs...)...)
		}
		if strings.TrimSpace(contractPath) != "" {
			cloned.RequiredArtifacts = append(cloned.RequiredArtifacts, contractPath)
		}
		cloned.RequiredArtifacts = uniqueStrings(cloned.RequiredArtifacts)
		return &cloned
	}

	requiredChecks := append([]string(nil), contract.RequiredStepTypes...)
	requiredChecks = append(requiredChecks, contract.EvaluatorCriteria...)
	requiredArtifacts := append([]string(nil), contract.PlanningArtifactRefs...)
	if strings.TrimSpace(contractPath) != "" {
		requiredArtifacts = append(requiredArtifacts, contractPath)
	}
	return &domain.VerificationContract{
		Version:           contract.Version,
		Goal:              firstNonEmptyValue(&planning, func(p *domain.PlanningArtifact) string { return p.Goal }, job.Goal),
		Scope:             append([]string(nil), contract.AcceptanceCriteria...),
		RequiredCommands:  append([]string(nil), contract.TesterInstructions...),
		RequiredArtifacts: uniqueStrings(requiredArtifacts),
		RequiredChecks:    uniqueStrings(requiredChecks),
		DisallowedActions: []string{"self-approval", "unbounded parallel fan-out"},
		MaxSeconds:        0,
		Notes:             contract.Summary,
		OwnerRole:         domain.RoleEvaluator,
	}
}

func verificationContractPath(job domain.Job) string {
	for _, ref := range job.PlanningArtifacts {
		if filepath.Base(ref) == "verification_contract.json" {
			return ref
		}
	}
	return ""
}

func loadVerificationContract(job domain.Job) (VerificationContract, string, error) {
	path := verificationContractPath(job)
	if strings.TrimSpace(path) == "" {
		return VerificationContract{}, "", fmt.Errorf("verification contract is not available")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return VerificationContract{}, path, err
	}
	var contract VerificationContract
	if err := json.Unmarshal(data, &contract); err != nil {
		return VerificationContract{}, path, err
	}
	return contract, path, nil
}

func verificationContractPrompt(contract VerificationContract, path string) string {
	var b strings.Builder
	b.WriteString("Verification contract ref: ")
	b.WriteString(path)
	b.WriteString("\n")
	b.WriteString("Verification contract summary: ")
	b.WriteString(contract.Summary)
	b.WriteString("\n")
	if len(contract.RequiredStepTypes) > 0 {
		b.WriteString("Required steps: ")
		b.WriteString(strings.Join(contract.RequiredStepTypes, ", "))
		b.WriteString("\n")
	}
	if len(contract.AcceptanceCriteria) > 0 {
		b.WriteString("Acceptance criteria:\n")
		for _, item := range contract.AcceptanceCriteria {
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
	}
	if len(contract.TesterInstructions) > 0 {
		b.WriteString("Tester instructions:\n")
		for _, item := range contract.TesterInstructions {
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func latestTestStep(job *domain.Job) *domain.Step {
	for i := len(job.Steps) - 1; i >= 0; i-- {
		step := &job.Steps[i]
		if strings.EqualFold(step.TaskType, "test") {
			return step
		}
	}
	return nil
}

func verificationSatisfied(job domain.Job, contract VerificationContract) (bool, []string) {
	missing := append([]string(nil), missingRequiredSteps(&job, contract.RequiredStepTypes)...)
	step := latestTestStep(&job)
	if step == nil {
		missing = append(missing, "test step")
		return false, uniqueStrings(missing)
	}
	if len(step.Artifacts) == 0 {
		missing = append(missing, "test artifacts")
	}
	if strings.TrimSpace(step.Summary) == "" {
		missing = append(missing, "tester summary")
	}
	return len(missing) == 0, uniqueStrings(missing)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
