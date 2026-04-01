package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPolicyAllowsAndBlocksByCategory(t *testing.T) {
	t.Parallel()

	policy := NewDefaultPolicy()

	if err := policy.Allows(Request{Category: CategoryBuild, Command: "go"}); err != nil {
		t.Fatalf("expected go to be allowed for build: %v", err)
	}
	if err := policy.Allows(Request{Category: CategorySearch, Command: "rg"}); err != nil {
		t.Fatalf("expected rg to be allowed for search: %v", err)
	}
	if err := policy.Allows(Request{Category: CategoryBuild, Command: "powershell"}); err == nil {
		t.Fatal("expected powershell to be blocked")
	}
}

func TestRunnerCapturesStdoutStderrAndExitCode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTempGoProgram(t, dir, `
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("stdout-line")
	fmt.Fprintln(os.Stderr, "stderr-line")
	os.Exit(7)
}
`)

	policy := NewDefaultPolicy()
	policy.Allow(CategoryTest, "probe")
	runner := NewRunner(policy)

	buildResult, err := runner.Run(context.Background(), Request{
		Category: CategoryBuild,
		Command:  "go",
		Args:     []string{"build", "-o", "probe.exe", "main.go"},
		Dir:      dir,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("expected build to succeed, got error: %v", err)
	}
	if buildResult.ExitCode != 0 {
		t.Fatalf("expected build exit code 0, got %d", buildResult.ExitCode)
	}

	result, err := runner.Run(context.Background(), Request{
		Category: CategoryTest,
		Command:  filepath.Join(dir, "probe.exe"),
		Dir:      dir,
		Timeout:  30 * time.Second,
	})
	if err == nil {
		t.Fatal("expected non-zero exit error from probe binary")
	}
	if result.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "stdout-line") {
		t.Fatalf("stdout not captured: %q", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "stderr-line") {
		t.Fatalf("stderr not captured: %q", result.Stderr)
	}
	if result.TimedOut {
		t.Fatal("did not expect timeout")
	}
}

func TestRunnerTimeout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTempGoProgram(t, dir, `
package main

import "time"

func main() {
	time.Sleep(5 * time.Second)
}
`)

	runner := NewRunner(NewDefaultPolicy())
	result, err := runner.Run(context.Background(), Request{
		Category: CategoryBuild,
		Command:  "go",
		Args:     []string{"run", "main.go"},
		Dir:      dir,
		Timeout:  50 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !result.TimedOut {
		t.Fatal("expected timed out result")
	}
}

func writeTempGoProgram(t *testing.T, dir, source string) {
	t.Helper()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(source)+"\n"), 0o644); err != nil {
		t.Fatalf("write temp program: %v", err)
	}
}
