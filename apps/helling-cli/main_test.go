package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionSubcommandWritesVersion(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run([]string{"version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, errOut.String())
	}

	got := out.String()
	for _, want := range []string{"helling ", "commit:", "built:", "go:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected version output to contain %q, got: %q", want, got)
		}
	}
}

func TestUnknownSubcommandReturnsNonZero(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run([]string{"not-a-real-command"}, &out, &errOut)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown subcommand, got 0 (stdout=%q)", out.String())
	}
	if !strings.Contains(errOut.String(), "helling:") {
		t.Fatalf("expected error prefix in stderr, got: %q", errOut.String())
	}
}

func TestCompletionSubcommandExists(t *testing.T) {
	var out, errOut bytes.Buffer

	// Cobra auto-registers the `completion` subcommand. Asking it for bash output
	// must succeed, proving the command tree is wired.
	code := run([]string{"completion", "bash"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0 for `completion bash`, got %d (stderr=%q)", code, errOut.String())
	}

	if len(out.Bytes()) == 0 {
		t.Fatal("expected non-empty completion output")
	}
}
