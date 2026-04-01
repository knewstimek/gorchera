package provider

import (
	"context"
	"fmt"

	"gorechera/internal/domain"
)

type Adapter interface {
	Name() domain.ProviderName
	RunLeader(ctx context.Context, job domain.Job) (string, error)
	RunWorker(ctx context.Context, job domain.Job, task domain.LeaderOutput) (string, error)
}

type PlannerRunner interface {
	RunPlanner(ctx context.Context, job domain.Job) (string, error)
}

type EvaluatorRunner interface {
	RunEvaluator(ctx context.Context, job domain.Job) (string, error)
}

type PhaseAdapter interface {
	Adapter
	PlannerRunner
	EvaluatorRunner
}

type Registry struct {
	adapters map[domain.ProviderName]Adapter
}

func NewRegistry() *Registry {
	registry := &Registry{adapters: make(map[domain.ProviderName]Adapter)}
	registry.Register(NewCodexAdapter())
	registry.Register(NewClaudeAdapter())
	return registry
}

func (r *Registry) Register(adapter Adapter) {
	r.adapters[adapter.Name()] = adapter
}

func (r *Registry) Get(name domain.ProviderName) (Adapter, error) {
	adapter, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("provider %q is not registered", name)
	}
	return adapter, nil
}

type SessionManager struct {
	registry *Registry
}

func NewSessionManager(registry *Registry) *SessionManager {
	return &SessionManager{registry: registry}
}

func (m *SessionManager) RunLeader(ctx context.Context, job domain.Job) (string, error) {
	return m.runRole(ctx, job, domain.RoleLeader, func(adapter Adapter) (string, error) {
		return adapter.RunLeader(ctx, job)
	})
}

func (m *SessionManager) RunWorker(ctx context.Context, job domain.Job, task domain.LeaderOutput) (string, error) {
	role := domain.RoleForTaskType(task.TaskType)
	return m.runRole(ctx, job, role, func(adapter Adapter) (string, error) {
		return adapter.RunWorker(ctx, job, task)
	})
}

func (m *SessionManager) RunPlanner(ctx context.Context, job domain.Job) (string, error) {
	return m.runPhase(ctx, job, domain.RolePlanner, func(adapter Adapter) (string, error) {
		phase, ok := adapter.(PlannerRunner)
		if !ok {
			return "", unsupportedPhaseError(adapter.Name(), "", "planner")
		}
		return phase.RunPlanner(ctx, job)
	})
}

func (m *SessionManager) RunEvaluator(ctx context.Context, job domain.Job) (string, error) {
	return m.runPhase(ctx, job, domain.RoleEvaluator, func(adapter Adapter) (string, error) {
		phase, ok := adapter.(EvaluatorRunner)
		if !ok {
			return "", unsupportedPhaseError(adapter.Name(), "", "evaluator")
		}
		return phase.RunEvaluator(ctx, job)
	})
}

func (m *SessionManager) runRole(ctx context.Context, job domain.Job, role domain.RoleName, invoke func(Adapter) (string, error)) (string, error) {
	profile := m.resolveProfile(job, role)
	adapter, err := m.adapterForProfile(profile)
	if err != nil {
		return "", err
	}
	return invoke(adapter)
}

func (m *SessionManager) runPhase(ctx context.Context, job domain.Job, role domain.RoleName, invoke func(Adapter) (string, error)) (string, error) {
	profile := m.resolveProfile(job, role)
	adapter, err := m.adapterForProfile(profile)
	if err != nil {
		return "", err
	}
	return invoke(adapter)
}

func (m *SessionManager) resolveProfile(job domain.Job, role domain.RoleName) domain.ExecutionProfile {
	profile := job.RoleProfiles.ProfileFor(role, job.Provider)
	if profile.Provider == "" {
		profile.Provider = job.Provider
	}
	if profile.Provider == "" {
		profile.Provider = domain.ProviderMock
	}
	return profile
}

func (m *SessionManager) adapterForProfile(profile domain.ExecutionProfile) (Adapter, error) {
	if adapter, err := m.registry.Get(profile.Provider); err == nil {
		return adapter, nil
	} else if profile.FallbackProvider != "" && profile.FallbackProvider != profile.Provider {
		if fallback, fallbackErr := m.registry.Get(profile.FallbackProvider); fallbackErr == nil {
			return fallback, nil
		} else {
			return nil, fmt.Errorf("primary provider %s: %w; fallback provider %s: %v", profile.Provider, err, profile.FallbackProvider, fallbackErr)
		}
	} else {
		return nil, err
	}
}
