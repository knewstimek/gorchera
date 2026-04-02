package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorchera/internal/domain"
	runtimeexec "gorchera/internal/runtime"
)

type VerificationContract struct {
	Version              int                 `json:"version"`
	Goal                 string              `json:"goal"`
	Summary              string              `json:"summary"`
	SprintContractRef    string              `json:"sprint_contract_ref,omitempty"`
	PlanningArtifactRefs []string            `json:"planning_artifact_refs,omitempty"`
	RequiredStepTypes    []string            `json:"required_step_types,omitempty"`
	AcceptanceCriteria   []string            `json:"acceptance_criteria,omitempty"`
	EngineInstructions   []string            `json:"engine_instructions,omitempty"`
	EvaluatorCriteria    []string            `json:"evaluator_criteria,omitempty"`
	RubricAxes           []domain.RubricAxis `json:"rubric_axes,omitempty"`
}

type EngineCheckArtifact struct {
	Kind    string              `json:"kind"`
	Status  string              `json:"status"`
	Command string              `json:"command,omitempty"`
	Reason  string              `json:"reason,omitempty"`
	Result  *runtimeexec.Result `json:"result,omitempty"`
}

const (
	engineCheckPassed  = "passed"
	engineCheckFailed  = "failed"
	engineCheckSkipped = "skipped"
)

func buildVerificationContract(job domain.Job, planning domain.PlanningArtifact, sprint domain.SprintContract, planningArtifactRefs []string) VerificationContract {
	engineInstructions := []string{
		"Engine runs go build ./... and go test ./... after each successful implement step when a Go workspace is configured.",
		"Treat skipped engine checks as informational when the workspace is not configured for Go or the go tool is unavailable.",
		"Treat failed engine checks as unresolved regression evidence until a later implement step replaces them.",
	}
	evaluatorCriteria := []string{
		"latest successful implement step contains engine build/test evidence",
		"required step coverage matches the sprint contract",
		"review coverage matches the selected pipeline mode",
	}
	return VerificationContract{
		Version:              1,
		Goal:                 firstNonEmptyValue(&planning, func(p *domain.PlanningArtifact) string { return p.Goal }, job.Goal),
		Summary:              verificationSummary(job, planning, sprint),
		SprintContractRef:    job.SprintContractRef,
		PlanningArtifactRefs: append([]string(nil), planningArtifactRefs...),
		RequiredStepTypes:    append([]string(nil), sprint.RequiredStepTypes...),
		AcceptanceCriteria:   append([]string(nil), sprint.AcceptanceCriteria...),
		EngineInstructions:   engineInstructions,
		EvaluatorCriteria:    evaluatorCriteria,
	}
}

func verificationSummary(job domain.Job, planning domain.PlanningArtifact, sprint domain.SprintContract) string {
	required := strings.Join(sprint.RequiredStepTypes, ", ")
	if required == "" {
		required = "implement"
	}
	return fmt.Sprintf(
		"Verify %s with required worker steps: %s and engine-managed go build/go test evidence",
		firstNonEmptyValue(&planning, func(p *domain.PlanningArtifact) string { return p.Summary }, job.Goal),
		required,
	)
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
		RequiredCommands:  append([]string(nil), contract.EngineInstructions...),
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
		b.WriteString("Required worker steps: ")
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
	if len(contract.EngineInstructions) > 0 {
		b.WriteString("Engine verification instructions:\n")
		for _, item := range contract.EngineInstructions {
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func latestImplementStep(job *domain.Job) *domain.Step {
	for i := len(job.Steps) - 1; i >= 0; i-- {
		step := &job.Steps[i]
		if strings.EqualFold(step.TaskType, "implement") {
			return step
		}
	}
	return nil
}

func loadEngineCheckArtifacts(step *domain.Step) map[string]EngineCheckArtifact {
	if step == nil {
		return nil
	}
	out := make(map[string]EngineCheckArtifact, 2)
	for _, artifactPath := range step.Artifacts {
		base := strings.ToLower(filepath.Base(artifactPath))
		if !strings.Contains(base, "engine_build") && !strings.Contains(base, "engine_test") {
			continue
		}
		data, err := os.ReadFile(artifactPath)
		if err != nil {
			continue
		}
		var artifact EngineCheckArtifact
		if err := json.Unmarshal(data, &artifact); err != nil {
			continue
		}
		kind := strings.TrimSpace(strings.ToLower(artifact.Kind))
		if kind == "" {
			switch {
			case strings.Contains(base, "engine_build"):
				kind = "build"
			case strings.Contains(base, "engine_test"):
				kind = "test"
			}
		}
		if kind != "" {
			out[kind] = artifact
		}
	}
	return out
}

func engineVerificationSatisfied(step *domain.Step) (bool, []string) {
	if step == nil {
		return false, []string{"implement step"}
	}
	artifacts := loadEngineCheckArtifacts(step)
	// Legacy jobs (created before engine verification was introduced) have no
	// engine_build/engine_test artifacts at all. Skip verification rather than
	// blocking them forever with "build evidence"/"test evidence" missing.
	if len(artifacts) == 0 {
		hasEngineArtifact := false
		for _, artifactPath := range step.Artifacts {
			base := strings.ToLower(filepath.Base(artifactPath))
			if strings.Contains(base, "engine_build") || strings.Contains(base, "engine_test") {
				hasEngineArtifact = true
				break
			}
		}
		if !hasEngineArtifact {
			// No engine artifacts present at all -- legacy job, skip verification.
			return true, nil
		}
	}
	var missing []string
	for _, kind := range []string{"build", "test"} {
		artifact, ok := artifacts[kind]
		if !ok {
			missing = append(missing, kind+" evidence")
			continue
		}
		switch strings.TrimSpace(strings.ToLower(artifact.Status)) {
		case engineCheckPassed, engineCheckSkipped:
		default:
			if artifact.Reason != "" {
				missing = append(missing, fmt.Sprintf("%s failed: %s", kind, artifact.Reason))
			} else {
				missing = append(missing, kind+" failed")
			}
		}
	}
	return len(missing) == 0, uniqueStrings(missing)
}

func verificationSatisfied(job domain.Job, contract VerificationContract) (bool, []string) {
	missing := append([]string(nil), missingRequiredSteps(&job, contract.RequiredStepTypes)...)
	if ok, engineMissing := engineVerificationSatisfied(latestImplementStep(&job)); !ok {
		missing = append(missing, engineMissing...)
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
