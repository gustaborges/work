// Package integration drives the built `work` binary end to end with
// testscript. Each .txtar file is one scenario from quickstart.md.
package integration

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/gustaborges/work/internal/cli"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work/verify"
	"github.com/gustaborges/work/seed"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"work": cli.Execute,
	})
}

func TestScripts(t *testing.T) {
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed; run `make seed` (%v)", err)
	}

	hostPath := os.Getenv("PATH")

	testscript.Run(t, testscript.Params{
		Dir: ".",
		Setup: func(env *testscript.Env) error {
			home := filepath.Join(env.WorkDir, "home")
			if err := os.MkdirAll(home, 0o755); err != nil {
				return err
			}
			env.Setenv("HOME", home)
			env.Setenv("USERPROFILE", home)
			env.Setenv("PATH", hostPath)
			env.Setenv("WORK_HOME", filepath.Join(env.WorkDir, "dothome"))
			env.Setenv("GIT_AUTHOR_NAME", "t")
			env.Setenv("GIT_AUTHOR_EMAIL", "t@t")
			env.Setenv("GIT_COMMITTER_NAME", "t")
			env.Setenv("GIT_COMMITTER_EMAIL", "t@t")
			env.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(env.WorkDir, "gitconfig"))
			env.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// verifycoherent asserts internal/work/verify.Check passes for
			// every row in the projection database.
			"verifycoherent": func(ts *testscript.TestScript, neg bool, args []string) {
				dbPath := filepath.Join(ts.Getenv("WORK_HOME"), "state", "work.db")
				err := checkAll(dbPath)
				if neg && err == nil {
					ts.Fatalf("verifycoherent: expected a coherence failure")
				}
				if !neg && err != nil {
					ts.Fatalf("verifycoherent: %v", err)
				}
			},
			// work-set-workspace <config.json> <path> rewrites the
			// workspace field, standing in for a user editing work.json.
			"work-set-workspace": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 2 {
					ts.Fatalf("usage: work-set-workspace <config.json> <path>")
				}
				path := ts.MkAbs(args[0])
				cfg, err := config.Load(path)
				if err != nil {
					ts.Fatalf("load config: %v", err)
				}
				cfg.Workspace = args[1]
				if err := config.Save(path, cfg); err != nil {
					ts.Fatalf("save config: %v", err)
				}
			},
			// gitrepo <dir> initialises a repo with one commit on main.
			"gitrepo": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: gitrepo <dir>")
				}
				dir := ts.MkAbs(args[0])
				if err := os.MkdirAll(dir, 0o755); err != nil {
					ts.Fatalf("%v", err)
				}
				for _, c := range [][]string{
					{"init", "-q", "-b", "main"},
					{"commit", "-q", "--allow-empty", "-m", "init"},
				} {
					cmd := exec.Command("git", append([]string{"-C", dir}, c...)...)
					cmd.Env = environ(ts)
					if out, err := cmd.CombinedOutput(); err != nil {
						ts.Fatalf("git %s: %v\n%s", strings.Join(c, " "), err, out)
					}
				}
			},
			// seedworks <src> <slug>... runs `work start` once per slug against
			// the repo at <src>, into $WS, with a >1 s gap between runs so each
			// Work's last_accessed_at is distinct and the recency order is
			// unambiguous. The Work directory for slug <s> is
			// $WS/in-progress/<basename(src)>_<s>/. Used by the F2 resume and
			// archive scenarios.
			"seedworks": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) < 2 {
					ts.Fatalf("usage: seedworks <src> <slug>...")
				}
				src := ts.MkAbs(args[0])
				ws := ts.Getenv("WS")
				if ws == "" {
					ts.Fatalf("seedworks: $WS is not set")
				}
				for i, slug := range args[1:] {
					if i > 0 {
						time.Sleep(1100 * time.Millisecond)
					}
					err := ts.Exec("work", "start", src,
						"--workspace", ws, "--base", "main",
						"--slug", slug, "--prefix", "{slug}", "--yes")
					if err != nil {
						ts.Fatalf("seedworks %s: %v", slug, err)
					}
				}
			},
			// dirty <worktree-dir> writes an untracked file into a Work's
			// worktree so git status --porcelain reports it as dirty.
			"dirty": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: dirty <worktree-dir>")
				}
				dir := ts.MkAbs(args[0])
				if err := os.WriteFile(filepath.Join(dir, "UNTRACKED.txt"), []byte("dirty\n"), 0o644); err != nil {
					ts.Fatalf("dirty: %v", err)
				}
			},
			// prearchivedir <archived-root> <name> pre-creates
			// <archived-root>/<yyyymmdd>-<name>/ (today's date) so the archive
			// pathing collision-suffix path (-2, -3, ...) is exercised.
			"prearchivedir": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 2 {
					ts.Fatalf("usage: prearchivedir <archived-root> <name>")
				}
				p := filepath.Join(ts.MkAbs(args[0]), time.Now().Format("20060102")+"-"+args[1])
				if err := os.MkdirAll(p, 0o755); err != nil {
					ts.Fatalf("prearchivedir: %v", err)
				}
			},
		},
	})
}

func environ(ts *testscript.TestScript) []string {
	keys := []string{
		"PATH", "HOME", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL",
		"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL",
		"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM",
	}
	env := make([]string, 0, len(keys))
	for _, k := range keys {
		env = append(env, k+"="+ts.Getenv(k))
	}
	return env
}

func checkAll(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("no projection database at %s", dbPath)
	}
	db, err := projection.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.List()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("projection database has no rows")
	}
	for _, r := range rows {
		rep, err := verify.Check(db, r.ID)
		if err != nil {
			return err
		}
		if !rep.OK {
			return fmt.Errorf("work %s incoherent: %s", r.ID, strings.Join(rep.Problems, "; "))
		}
	}
	return nil
}
