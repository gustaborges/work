package shellintegration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnippetForEachShell(t *testing.T) {
	for _, sh := range Supported {
		s, err := Snippet(sh)
		if err != nil {
			t.Fatalf("Snippet(%q): %v", sh, err)
		}
		if !strings.Contains(s, "work") || !strings.Contains(s, "WORK_CD_FILE") {
			t.Errorf("Snippet(%q) missing wrapper essentials:\n%s", sh, s)
		}
	}
}

func TestSnippetUnknownShell(t *testing.T) {
	_, err := Snippet("frobnicate")
	if err == nil || !strings.Contains(err.Error(), "bash") {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteTargetPath(t *testing.T) {
	f := filepath.Join(t.TempDir(), "cd")
	t.Setenv("WORK_CD_FILE", f)
	if err := WriteTargetPath("/abs/worktree"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(f)
	if string(got) != "/abs/worktree" {
		t.Errorf("file = %q", got)
	}
}

func TestWriteTargetPathNoIntegration(t *testing.T) {
	t.Setenv("WORK_CD_FILE", "")
	if err := WriteTargetPath("/abs/worktree"); err != nil {
		t.Fatalf("want nil no-op, got %v", err)
	}
	if Active() {
		t.Error("Active() should be false")
	}
}

func TestReportNoIntegrationText(t *testing.T) {
	var b bytes.Buffer
	ReportNoIntegration(&b, "/abs/worktree", "zsh")
	out := b.String()
	for _, want := range []string{
		"note: this shell session was not moved into the new worktree.",
		"note: worktree path:",
		"/abs/worktree",
		`eval "$(work shell-init zsh)"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(strings.ToLower(out), "changed directory") {
		t.Error("must not claim a cd happened")
	}
}

func TestReportNoIntegrationUnknownShellFallsBackToBash(t *testing.T) {
	var b bytes.Buffer
	ReportNoIntegration(&b, "/abs/worktree", "")
	if !strings.Contains(b.String(), "shell-init bash") {
		t.Errorf("expected bash fallback:\n%s", b.String())
	}
}
