package provider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

func probeExecutable(ctx context.Context, executable string, timeout time.Duration, args ...string) (CommandResult, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if _, err := exec.LookPath(executable); err != nil {
		return CommandResult{}, err
	}

	cmd := exec.CommandContext(probeCtx, executable, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		if probeCtx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("probe timed out: %w", probeCtx.Err())
		}
		return result, err
	}
	return result, nil
}

// runExecutableWithStdin is like runExecutable but feeds stdinData to the process stdin.
func runExecutableWithStdin(ctx context.Context, executable string, timeout time.Duration, dir string, env []string, stdinData string, args ...string) (CommandResult, error) {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if _, err := exec.LookPath(executable); err != nil {
		return CommandResult{}, err
	}

	cmd := exec.CommandContext(runCtx, executable, args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	cmd.Stdin = strings.NewReader(stdinData)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		if runCtx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("provider command timed out: %w", runCtx.Err())
		}
		return result, err
	}
	return result, nil
}

func runExecutable(ctx context.Context, executable string, timeout time.Duration, dir string, env []string, args ...string) (CommandResult, error) {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if _, err := exec.LookPath(executable); err != nil {
		return CommandResult{}, err
	}

	cmd := exec.CommandContext(runCtx, executable, args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		if runCtx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("provider command timed out: %w", runCtx.Err())
		}
		return result, err
	}
	return result, nil
}
