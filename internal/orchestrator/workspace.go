package orchestrator

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gorchera/internal/domain"
)

func ValidateWorkspaceDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("workspace directory must be an absolute path: %s", path)
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			info, statErr := os.Lstat(path)
			if statErr == nil && info.Mode()&os.ModeSymlink == 0 {
				resolved = filepath.Clean(path)
				err = nil
			}
		}
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("workspace directory does not exist: %s", path)
		}
		return fmt.Errorf("resolve workspace directory %q: %w", path, err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("workspace directory does not exist: %s", path)
		}
		return fmt.Errorf("stat workspace directory %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace directory does not exist: %s", path)
	}
	return nil
}

func normalizeWorkspaceMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case string(domain.WorkspaceModeIsolated):
		return string(domain.WorkspaceModeIsolated)
	default:
		return string(domain.WorkspaceModeShared)
	}
}

func prepareWorkspaceDir(workspaceRoot, requestedWorkspaceDir, jobID, workspaceMode string) (string, string, string, error) {
	sourceWorkspaceDir := firstNonEmpty(strings.TrimSpace(requestedWorkspaceDir), workspaceRoot)
	if err := ValidateWorkspaceDir(sourceWorkspaceDir); err != nil {
		return "", "", "", err
	}

	normalizedMode := normalizeWorkspaceMode(workspaceMode)
	if normalizedMode != string(domain.WorkspaceModeIsolated) {
		return sourceWorkspaceDir, sourceWorkspaceDir, normalizedMode, nil
	}

	isolatedWorkspaceDir, err := createGitWorktreeWorkspace(workspaceRoot, sourceWorkspaceDir, jobID)
	if err != nil {
		return "", "", "", err
	}
	if err := ValidateWorkspaceDir(isolatedWorkspaceDir); err != nil {
		return "", "", "", err
	}
	return isolatedWorkspaceDir, sourceWorkspaceDir, normalizedMode, nil
}

func createGitWorktreeWorkspace(workspaceRoot, sourceWorkspaceDir, jobID string) (string, error) {
	repoRoot, err := gitOutput(sourceWorkspaceDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("isolated workspace mode requires a git repository: %w", err)
	}
	repoRoot = filepath.Clean(repoRoot)

	relativeWorkspace, err := filepath.Rel(repoRoot, sourceWorkspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve isolated workspace subdir: %w", err)
	}

	_ = workspaceRoot
	worktreeParent := filepath.Join(filepath.Dir(repoRoot), ".gorchera-worktrees", filepath.Base(repoRoot))
	worktreeDir := filepath.Join(worktreeParent, jobID)
	if err := os.MkdirAll(worktreeParent, 0o755); err != nil {
		return "", fmt.Errorf("create isolated workspace parent: %w", err)
	}
	if _, err := os.Stat(worktreeDir); err == nil {
		return "", fmt.Errorf("isolated workspace already exists for job %s: %s", jobID, worktreeDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat isolated workspace %q: %w", worktreeDir, err)
	}

	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "--detach", worktreeDir, "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("create isolated git worktree: %w: %s", err, strings.TrimSpace(string(output)))
	}

	if relativeWorkspace == "." {
		return worktreeDir, nil
	}
	return filepath.Join(worktreeDir, relativeWorkspace), nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
