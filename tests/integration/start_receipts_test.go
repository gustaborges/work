//go:build unix

package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"
)

// A guided `work start` where the path step is rejected four times and the slug
// step twice: the reconstructed terminal shows one receipt per accepted step
// and none of the rejected inputs or their errors, while stdout still carries
// the byte-for-byte F1 success lines. (US1 #1, #2; SC-001, SC-002.)
func TestStartReceiptsLeaveNoDebris(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "src")
	makeRepo(t, repo)
	gitIn(t, repo, "branch", "taken")
	// ValidatePath displays the absolute, symlink-resolved repository path.
	// macOS commonly exposes the temporary directory as /var while its
	// canonical path is /private/var.
	displayRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("resolve repository path for receipt: %v", err)
	}

	c := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 80}, bin, env, "start")

	// Path: four rejects, each replacing the last error in-frame, then the repo.
	c.expect("Local repository path")
	c.send("/no/such/a\r")
	c.expect("does not exist")
	c.send("\x15/no/such/b\r")
	c.expect("does not exist")
	c.send("\x15/no/such/c\r")
	c.expect("does not exist")
	c.send("\x15/tmp\r") // exists, but not a git repository
	c.expect("not a Git repository")
	c.send("\x15" + repo + "\r")

	// Slug: invalid, then colliding, then valid.
	c.expect("Slug")
	c.send("bad slug\r")
	c.expect("whitespace")
	c.send("\x15taken\r")
	c.expect("already exists")
	c.send("\x15my-work\r")

	// Base branch: the only branch is main; accept it.
	c.expect("Base branch")
	c.send("\r")

	// Workspace root: accept the suggested default.
	c.expect("Workspace root")
	c.send("\r")

	// Confirmation.
	c.expect("Create Work")
	c.send("y")
	c.expect("work: created ")

	if code := c.wait(); code != 0 {
		t.Fatalf("guided start exited %d, want 0\n%s", code, c.screen())
	}

	screen := c.screen()

	// One receipt per accepted step.
	for _, want := range []string{
		"Local repository path\n  \u2714 " + displayRepo,
		"Slug\n  \u2714 my-work",
		"Base branch\n  \u2714 main",
		"\u2714 Create Work confirmed",
	} {
		if !strings.Contains(screen, want) {
			t.Errorf("reconstructed screen missing receipt %q:\n%s", want, screen)
		}
	}

	// No rejected input or stale error survived into the visible terminal.
	for _, gone := range []string{
		"/no/such/a", "/no/such/b", "/no/such/c",
		"does not exist", "not a Git repository",
		"bad slug", "whitespace", "already exists",
	} {
		if strings.Contains(screen, gone) {
			t.Errorf("reconstructed screen still shows rejected content %q:\n%s", gone, screen)
		}
	}

	// The stable F1 stdout contract is unchanged.
	for _, want := range []string{"work: created ", "work: branch ", "work: path "} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen missing stable stdout line %q:\n%s", want, screen)
		}
	}
}
