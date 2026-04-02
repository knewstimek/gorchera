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

func TestRunRecoveryScenarioCompletesSeededJobs(t *testing.T) {
	t.Parallel()

	serverBin := buildGorcheraBinary(t)
	workdir := t.TempDir()

	summary, err := Run(Config{
		ServerBin:     serverBin,
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
