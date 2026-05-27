package kovaloopcli

import (
	"bytes"
	"testing"
)

func TestVersionCommandsPrintVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exitCode := Run(args, &stdout, &stderr, EnvMap{})
		if exitCode != 0 {
			t.Fatalf("Run(%v) exit code = %d stderr=%q", args, exitCode, stderr.String())
		}
		if stdout.String() != "kovaloop "+CLIVersion+"\n" {
			t.Fatalf("stdout = %q", stdout.String())
		}
		if stderr.String() != "" {
			t.Fatalf("stderr = %q", stderr.String())
		}
	}
}

func TestUnknownCommandPrintsUsageAndReturnsTwo(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"bogus"}, &stdout, &stderr, EnvMap{})
	if exitCode != 2 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Kovaloop CLI for ZeroClaw")) {
		t.Fatalf("usage missing from stderr: %q", stderr.String())
	}
}

func TestLedgerHelpPrintsUsageToStdout(t *testing.T) {
	for _, args := range [][]string{
		{"ledger", "help"},
		{"ledger", "-h"},
		{"ledger", "--help"},
	} {
		t.Run(args[1], func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Run(args, &stdout, &stderr, EnvMap{})
			if exitCode != 0 {
				t.Fatalf("exit code = %d", exitCode)
			}
			if !bytes.Contains(stdout.Bytes(), []byte("Kovaloop CLI for ZeroClaw")) {
				t.Fatalf("usage missing from stdout: %q", stdout.String())
			}
			if stderr.String() != "" {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestBadArgsPrintUsageToStderr(t *testing.T) {
	for _, args := range [][]string{
		{"claim"},
		{"ledger"},
		{"ledger", "bogus"},
		{"ledger", "escrow"},
		{"ledger", "escrow", "bogus"},
	} {
		t.Run(args[0], func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Run(args, &stdout, &stderr, EnvMap{})
			if exitCode != 2 {
				t.Fatalf("exit code = %d", exitCode)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q", stdout.String())
			}
			if !bytes.Contains(stderr.Bytes(), []byte("Kovaloop CLI for ZeroClaw")) {
				t.Fatalf("usage missing from stderr: %q", stderr.String())
			}
		})
	}
}
