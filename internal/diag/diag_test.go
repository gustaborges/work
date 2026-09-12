package diag

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCategoryTable(t *testing.T) {
	want := []struct {
		cat   Category
		token string
		code  int
	}{
		{OK, "ok", 0},
		{Usage, "usage", 2},
		{InvalidPath, "invalid-path", 10},
		{UnusableRepo, "unusable-repo", 11},
		{NoBaseBranch, "no-base-branch", 12},
		{InvalidBranchName, "invalid-branch-name", 13},
		{BranchCollision, "branch-collision", 14},
		{DestinationUnavailable, "destination-unavailable", 15},
		{BootstrapFailed, "bootstrap-failed", 16},
		{MaterializationFailed, "materialization-failed", 17},
		{Cancelled, "cancelled", 20},
		{TargetNotFound, "target-not-found", 21},
		{TargetArchived, "target-archived", 22},
		{DirtyWorktree, "dirty-worktree", 23},
		{ArchiveFailed, "archive-failed", 24},
		{SnapshotUnreadable, "snapshot-unreadable", 25},
		{NoRepositoryFound, "no-repository-found", 26},
		{NoEligibleLocator, "no-eligible-locator", 27},
		{RepositoryCandidateInvalid, "repository-candidate-invalid", 28},
		{LocatorFailed, "locator-failed", 29},
		{RepositoryAmbiguous, "repository-ambiguous", 30},
	}

	if len(want) != len(All) {
		t.Fatalf("All has %d entries, table has %d", len(All), len(want))
	}
	for i, w := range want {
		if w.cat.Token != w.token {
			t.Errorf("row %d: token = %q, want %q", i, w.cat.Token, w.token)
		}
		if w.cat.Code != w.code {
			t.Errorf("row %d (%s): code = %d, want %d", i, w.token, w.cat.Code, w.code)
		}
		if All[i] != w.cat {
			t.Errorf("All[%d] = %+v, want %+v", i, All[i], w.cat)
		}
	}
}

func TestTokensUniqueAndCodesUnique(t *testing.T) {
	tokens := map[string]bool{}
	codes := map[int]bool{}
	for _, c := range All {
		if tokens[c.Token] {
			t.Errorf("duplicate token %q", c.Token)
		}
		if codes[c.Code] {
			t.Errorf("duplicate code %d", c.Code)
		}
		tokens[c.Token] = true
		codes[c.Code] = true
	}
}

func TestExitCode(t *testing.T) {
	if got := ExitCode(nil); got != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", got)
	}
	if got := ExitCode(errors.New("plain")); got != 1 {
		t.Errorf("ExitCode(plain) = %d, want 1", got)
	}
	if got := ExitCode(New(BranchCollision, "x")); got != 14 {
		t.Errorf("ExitCode(branch-collision) = %d, want 14", got)
	}
	wrapped := fmt.Errorf("context: %w", New(InvalidPath, "x"))
	if got := ExitCode(wrapped); got != 10 {
		t.Errorf("ExitCode(wrapped) = %d, want 10", got)
	}
}

func TestFormatAndToken(t *testing.T) {
	err := New(UnusableRepo, "the path /home/u/x is not a git repository")
	got := Format(err)
	want := "error: unusable-repo: the path /home/u/x is not a git repository"
	if got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
	if Token(err) != "unusable-repo" {
		t.Errorf("Token = %q", Token(err))
	}
	if Token(errors.New("plain")) != "error" {
		t.Errorf("Token(plain) = %q, want error", Token(errors.New("plain")))
	}
}

func TestSummaryAndHintDoNotChangeStableOutput(t *testing.T) {
	// An Error with no Summary/Hint formats and codes exactly as before.
	plain := New(InvalidPath, "no such path: /x")
	if got, want := Format(plain), "error: invalid-path: no such path: /x"; got != want {
		t.Errorf("Format(plain) = %q, want %q", got, want)
	}

	// Adding Summary/Hint leaves Format, Token, and ExitCode untouched.
	rich := New(InvalidPath, "no such path: /x").
		WithSummary("that path does not exist").
		WithHint("pass a path to a local git repository")
	if got := Format(rich); got != Format(plain) {
		t.Errorf("Format changed with Summary/Hint: %q", got)
	}
	if Token(rich) != "invalid-path" {
		t.Errorf("Token changed: %q", Token(rich))
	}
	if ExitCode(rich) != 10 {
		t.Errorf("ExitCode changed: %d", ExitCode(rich))
	}
	if rich.Summary != "that path does not exist" || rich.Hint != "pass a path to a local git repository" {
		t.Errorf("Summary/Hint not stored: %+v", rich)
	}
}

func TestCauseAliasesUnwrap(t *testing.T) {
	cause := errors.New("boom")
	err := Wrap(BootstrapFailed, cause, "cannot start")
	if err.Cause() != cause || err.Cause() != err.Unwrap() {
		t.Errorf("Cause() = %v, want %v (== Unwrap)", err.Cause(), cause)
	}
	if New(Usage, "x").Cause() != nil {
		t.Errorf("Cause() on a causeless Error should be nil")
	}
}

func TestWrapKeepsCauseHidden(t *testing.T) {
	cause := errors.New("permission denied: /etc/shadow")
	err := Wrap(BootstrapFailed, cause, "cannot read config file work.json")
	if strings.Contains(err.Error(), "shadow") {
		t.Errorf("user message leaks cause: %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(err, cause) = false, want true")
	}
}
