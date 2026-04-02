package api

import (
	"gorchera/internal/domain"
	runtimeexec "gorchera/internal/runtime"
)

type StartJobRequest struct {
	Goal          string              `json:"goal"`
	TechStack     string              `json:"tech_stack,omitempty"`
	WorkspaceDir  string              `json:"workspace_dir,omitempty"`
	WorkspaceMode string              `json:"workspace_mode,omitempty"`
	Constraints   []string            `json:"constraints,omitempty"`
	DoneCriteria  []string            `json:"done_criteria,omitempty"`
	Provider      domain.ProviderName `json:"provider,omitempty"`
	RoleProfiles  domain.RoleProfiles `json:"role_profiles,omitempty"`
	MaxSteps      int                 `json:"max_steps,omitempty"`
}

type CancelJobRequest struct {
	Reason string `json:"reason,omitempty"`
}

type RejectJobRequest struct {
	Reason string `json:"reason,omitempty"`
}

type SteerJobRequest struct {
	Message string `json:"message"`
}

type StartHarnessProcessRequest struct {
	Name           string               `json:"name,omitempty"`
	Category       runtimeexec.Category `json:"category,omitempty"`
	Command        string               `json:"command"`
	Args           []string             `json:"args,omitempty"`
	Dir            string               `json:"dir,omitempty"`
	Env            []string             `json:"env,omitempty"`
	TimeoutSeconds int                  `json:"timeout_seconds,omitempty"`
	MaxOutputBytes int64                `json:"max_output_bytes,omitempty"`
	LogDir         string               `json:"log_dir,omitempty"`
	Port           int                  `json:"port,omitempty"`
}
