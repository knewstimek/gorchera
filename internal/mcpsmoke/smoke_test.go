package mcpsmoke

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gorchera/internal/domain"
)

func TestRunBasicScenarioCompletesJob(t *testing.T) {
	t.Parallel()

	serverBin := buildGorcheraBinary(t)
	workdir := t.TempDir()

	summary, err := Run(Config{
		ServerBin:   serverBin,
		Workdir:     workdir,
		Scenario:    "basic",
		KeepWorkdir: true,
		WaitTimeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("Run(basic) returned error: %v", err)
	}
	if summary.ServerName != "gorchera" {
		t.Fatalf("server name = %q, want gorchera", summary.ServerName)
	}
	if summary.ToolCount == 0 {
		t.Fatal("expected non-zero tool count")
	}
	if summary.StartedJobStatus != string(domain.JobStatusDone) {
		t.Fatalf("started job status = %q, want done", summary.StartedJobStatus)
	}
}

func TestRunIsolatedScenarioCreatesDetachedWorkspace(t *testing.T) {
	t.Parallel()

	serverBin := buildGorcheraBinary(t)
	workdir := t.TempDir()

	summary, err := Run(Config{
		ServerBin:   serverBin,
		Workdir:     workdir,
		Scenario:    "isolated",
		KeepWorkdir: true,
		WaitTimeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("Run(isolated) returned error: %v", err)
	}
	if summary.StartedJobStatus != string(domain.JobStatusDone) {
		t.Fatalf("started job status = %q, want done", summary.StartedJobStatus)
	}
	if summary.WorkspaceMode != string(domain.WorkspaceModeIsolated) {
		t.Fatalf("workspace mode = %q, want isolated", summary.WorkspaceMode)
	}
	if summary.RequestedWorkspace == "" || summary.ActualWorkspace == "" {
		t.Fatalf("expected workspace paths in summary, got %#v", summary)
	}
	if filepath.Clean(summary.RequestedWorkspace) == filepath.Clean(summary.ActualWorkspace) {
		t.Fatalf("expected detached workspace, both paths = %q", summary.ActualWorkspace)
	}
	if !strings.Contains(summary.ActualWorkspace, filepath.Join(".gorchera-worktrees", filepath.Base(summary.RequestedWorkspace))) {
		t.Fatalf("expected actual workspace under .gorchera-worktrees, got %q", summary.ActualWorkspace)
	}
}

func TestRunRecoveryScenarioCompletesSeededJobs(t *testing.T) {
	t.Parallel()

	serverBin := buildGorcheraBinary(t)
	workdir := t.TempDir()

	summary, err := Run(Config{
		ServerBin:     serverBin,
		ServerArgs:    []string{"-recover"},
		Workdir:       workdir,
		Scenario:      "recovery",
		RecoveryJobs:  3,
		KeepWorkdir:   true,
		WaitTimeout:   20 * time.Second,
		RecoveryState: domain.JobStatusStarting,
	})
	if err != nil {
		t.Fatalf("Run(recovery) returned error: %v", err)
	}
	if summary.RecoveryRequested != 3 {
		t.Fatalf("recovery requested = %d, want 3", summary.RecoveryRequested)
	}
	if len(summary.RecoveredStatuses) != 3 {
		t.Fatalf("recovered statuses count = %d, want 3", len(summary.RecoveredStatuses))
	}
	for jobID, status := range summary.RecoveredStatuses {
		if status != string(domain.JobStatusDone) {
			t.Fatalf("recovered job %s status = %q, want done", jobID, status)
		}
	}
	if !strings.Contains(summary.Stderr, "scheduled 3 jobs with max concurrency 2") {
		t.Fatalf("expected recovery scheduling log, stderr=%q", summary.Stderr)
	}
}

func TestRunInterruptScenarioBlocksSeededStaleJobsByDefault(t *testing.T) {
	t.Parallel()

	serverBin := buildGorcheraBinary(t)
	workdir := t.TempDir()

	summary, err := Run(Config{
		ServerBin:     serverBin,
		Workdir:       workdir,
		Scenario:      "interrupt",
		RecoveryJobs:  3,
		KeepWorkdir:   true,
		WaitTimeout:   20 * time.Second,
		RecoveryState: domain.JobStatusWaitingLeader,
	})
	if err != nil {
		t.Fatalf("Run(interrupt) returned error: %v", err)
	}
	if len(summary.InterruptedStatuses) != 3 {
		t.Fatalf("interrupted statuses count = %d, want 3", len(summary.InterruptedStatuses))
	}
	for jobID, status := range summary.InterruptedStatuses {
		if status != string(domain.JobStatusBlocked) {
			t.Fatalf("interrupted job %s status = %q, want blocked", jobID, status)
		}
	}
	if !strings.Contains(summary.Stderr, "interrupt sweep: blocked 3 recoverable jobs") {
		t.Fatalf("expected interrupt sweep log, stderr=%q", summary.Stderr)
	}
}

func TestRunRecoveryScenarioSupportsSelectedStartupRecovery(t *testing.T) {
	t.Parallel()

	serverBin := buildGorcheraBinary(t)
	workdir := t.TempDir()

	summary, err := Run(Config{
		ServerBin:     serverBin,
		ServerArgs:    []string{"-recover-jobs", "recovery-job-02"},
		Workdir:       workdir,
		Scenario:      "recovery",
		RecoveryJobs:  3,
		RecoverJobIDs: []string{"recovery-job-02"},
		KeepWorkdir:   true,
		WaitTimeout:   20 * time.Second,
		RecoveryState: domain.JobStatusStarting,
	})
	if err != nil {
		t.Fatalf("Run(recovery selected) returned error: %v", err)
	}
	if got := summary.RecoveredStatuses["recovery-job-02"]; got != string(domain.JobStatusDone) {
		t.Fatalf("selected job status = %q, want done", got)
	}
	if got := summary.RecoveredStatuses["recovery-job-01"]; got != string(domain.JobStatusStarting) {
		t.Fatalf("unselected job 01 status = %q, want starting", got)
	}
	if got := summary.RecoveredStatuses["recovery-job-03"]; got != string(domain.JobStatusStarting) {
		t.Fatalf("unselected job 03 status = %q, want starting", got)
	}
	if !strings.Contains(summary.Stderr, "scheduled 1 selected jobs with max concurrency 2") {
		t.Fatalf("expected selected recovery scheduling log, stderr=%q", summary.Stderr)
	}
}

func buildGorcheraBinary(t *testing.T) string {
	t.Helper()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	binName := "gorchera-smoke-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	output := filepath.Join(t.TempDir(), binName)

	cmd := exec.Command("go", "build", "-o", output, "./cmd/gorchera")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/gorchera failed: %v\n%s", err, string(out))
	}
	return output
}
