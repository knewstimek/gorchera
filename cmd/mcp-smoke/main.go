package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorchera/internal/domain"
	"gorchera/internal/mcpsmoke"
)

func main() {
	fs := flag.NewFlagSet("mcp-smoke", flag.ExitOnError)
	serverBin := fs.String("server-bin", "", "path to the gorchera binary to launch")
	workdir := fs.String("workdir", "", "working directory for isolated smoke state")
	scenario := fs.String("scenario", "basic", "scenario: basic | recovery")
	recoveryJobs := fs.Int("recovery-jobs", 3, "number of recoverable jobs to seed for recovery scenario")
	keepWorkdir := fs.Bool("keep-workdir", false, "keep the smoke workdir after completion")
	waitTimeout := fs.Duration("wait-timeout", 20*time.Second, "per-call MCP timeout")
	recoveryState := fs.String("recovery-state", string(domain.JobStatusStarting), "seeded recovery state: starting | running | waiting_leader | waiting_worker")
	fs.Parse(os.Args[1:])

	if *serverBin == "" {
		fmt.Fprintln(os.Stderr, "mcp-smoke requires -server-bin")
		os.Exit(2)
	}

	if *workdir == "" {
		tmp, err := os.MkdirTemp("", "gorchera-mcp-smoke-*")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		*workdir = tmp
	}

	summary, err := mcpsmoke.Run(mcpsmoke.Config{
		ServerBin:     absOrSelf(*serverBin),
		Workdir:       absOrSelf(*workdir),
		Scenario:      *scenario,
		RecoveryJobs:  *recoveryJobs,
		KeepWorkdir:   *keepWorkdir,
		WaitTimeout:   *waitTimeout,
		RecoveryState: domain.JobStatus(*recoveryState),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	data, err := json.Marshal(summary)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func absOrSelf(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
