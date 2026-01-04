package main

import "testing"

func TestParseSyncArgsAllowsInterspersedFlags(t *testing.T) {
	args := []string{"/remote", "/local", "--report", "out.json", "--delete", "--dry-run"}
	parsed, err := parseSyncArgs(args)
	if err != nil {
		t.Fatalf("parseSyncArgs error: %v", err)
	}
	if !parsed.DryRun {
		t.Fatalf("expected DryRun true")
	}
	if !parsed.Delete {
		t.Fatalf("expected Delete true")
	}
	if parsed.ReportPath != "out.json" {
		t.Fatalf("expected report path out.json, got %q", parsed.ReportPath)
	}
	if len(parsed.Positional) != 2 {
		t.Fatalf("expected 2 positional args, got %d", len(parsed.Positional))
	}
}

func TestParseSyncArgsReportEquals(t *testing.T) {
	args := []string{"--report=out.json", "/remote", "/local"}
	parsed, err := parseSyncArgs(args)
	if err != nil {
		t.Fatalf("parseSyncArgs error: %v", err)
	}
	if parsed.ReportPath != "out.json" {
		t.Fatalf("expected report path out.json, got %q", parsed.ReportPath)
	}
	if len(parsed.Positional) != 2 {
		t.Fatalf("expected 2 positional args, got %d", len(parsed.Positional))
	}
}
