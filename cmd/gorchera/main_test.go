package main

import "testing"

func TestParseServeOptionsDefaultsRecoveryOff(t *testing.T) {
	t.Parallel()

	sopts, err := parseServeOptions(nil)
	if err != nil {
		t.Fatalf("parseServeOptions returned error: %v", err)
	}
	if sopts.addr != "127.0.0.1:8080" {
		t.Fatalf("addr = %q, want default listen address", sopts.addr)
	}
	if sopts.recover.enabled {
		t.Fatal("expected startup recovery to default off for serve")
	}
	if len(sopts.recover.jobIDs) != 0 {
		t.Fatalf("expected no selected job IDs, got %#v", sopts.recover.jobIDs)
	}
}

func TestParseServeOptionsEnablesSelectedRecovery(t *testing.T) {
	t.Parallel()

	sopts, err := parseServeOptions([]string{"-addr", "127.0.0.1:9090", "-recover-jobs", "job-a, job-b"})
	if err != nil {
		t.Fatalf("parseServeOptions returned error: %v", err)
	}
	if sopts.addr != "127.0.0.1:9090" {
		t.Fatalf("addr = %q, want overridden listen address", sopts.addr)
	}
	if !sopts.recover.enabled {
		t.Fatal("expected startup recovery enabled when recover-jobs is set")
	}
	if len(sopts.recover.jobIDs) != 2 || sopts.recover.jobIDs[0] != "job-a" || sopts.recover.jobIDs[1] != "job-b" {
		t.Fatalf("unexpected selected job IDs: %#v", sopts.recover.jobIDs)
	}
}

func TestParseMCPOptionsDefaultsRecoveryOff(t *testing.T) {
	t.Parallel()

	opts, err := parseMCPOptions(nil)
	if err != nil {
		t.Fatalf("parseMCPOptions returned error: %v", err)
	}
	if opts.enabled {
		t.Fatal("expected startup recovery to default off for mcp")
	}
}

func TestParseMCPOptionsRecoverFlagAndJobs(t *testing.T) {
	t.Parallel()

	opts, err := parseMCPOptions([]string{"-recover", "-recover-jobs", "job-1"})
	if err != nil {
		t.Fatalf("parseMCPOptions returned error: %v", err)
	}
	if !opts.enabled {
		t.Fatal("expected startup recovery enabled")
	}
	if len(opts.jobIDs) != 1 || opts.jobIDs[0] != "job-1" {
		t.Fatalf("unexpected selected job IDs: %#v", opts.jobIDs)
	}
}
