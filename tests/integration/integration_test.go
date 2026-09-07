// Package integration drives the built `work` binary end to end with
// testscript. Each .txtar file is one scenario from quickstart.md.
package integration

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/gustaborges/work/internal/cli"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
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
					// --workspace is accepted only until the root is persisted;
					// pass it on the first run and let the rest inherit it.
					call := []string{"start", src, "--base", "main",
						"--slug", slug, "--prefix", "{slug}", "--yes"}
					if i == 0 {
						call = append(call, "--workspace", ws)
					}
					if err := ts.Exec("work", call...); err != nil {
						ts.Fatalf("seedworks %s: %v", slug, err)
					}
				}
			},
			// sleep <seconds> pauses the script. Resume ordering is keyed on
			// second-precision timestamps (ties broken by creation order), so a
			// scenario that needs a resume to out-sort a just-created Work waits
			// a second first — the same `sleep 1` the quickstart uses.
			"sleep": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: sleep <seconds>")
				}
				n, err := strconv.Atoi(args[0])
				if err != nil {
					ts.Fatalf("sleep: %v", err)
				}
				time.Sleep(time.Duration(n) * time.Second)
			},
			// workid <work-state.json> <envvar> reads work.id from the snapshot
			// and binds it to the named script environment variable, so a later
			// `work resume <id>` can target it.
			"workid": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 2 {
					ts.Fatalf("usage: workid <work-state.json> <envvar>")
				}
				st, err := work.Read(ts.MkAbs(args[0]))
				if err != nil {
					ts.Fatalf("workid: %v", err)
				}
				ts.Setenv(args[1], st.Work.ID)
			},
			// archivesnap <work-state.json> flips a snapshot to status=archived
			// in place (status + archived_at + last_accessed_at = now). It stands
			// in for `work archive` (Phase 4) so the resume-of-an-archived-Work
			// refusal can be exercised now: the next `work` command reconciles
			// the projection from the snapshot.
			"archivesnap": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: archivesnap <work-state.json>")
				}
				p := ts.MkAbs(args[0])
				st, err := work.Read(p)
				if err != nil {
					ts.Fatalf("archivesnap: %v", err)
				}
				st.Archive(time.Now().UTC())
				if err := work.Write(p, st); err != nil {
					ts.Fatalf("archivesnap: %v", err)
				}
			},
			// activeorder <slug>... asserts projection.ListActive() returns rows
			// whose slugs are exactly the given sequence (the recency order, most
			// recent first). Backs SC-002.
			"activeorder": func(ts *testscript.TestScript, neg bool, args []string) {
				dbPath := filepath.Join(ts.Getenv("WORK_HOME"), "state", "work.db")
				db, err := projection.Open(dbPath)
				if err != nil {
					ts.Fatalf("activeorder: open db: %v", err)
				}
				defer db.Close()
				rows, err := db.ListActive()
				if err != nil {
					ts.Fatalf("activeorder: %v", err)
				}
				got := make([]string, len(rows))
				for i, r := range rows {
					got[i] = r.Slug
				}
				want := strings.Join(args, ",")
				if strings.Join(got, ",") != want {
					ts.Fatalf("active order = [%s], want [%s]", strings.Join(got, ","), want)
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
			// today <envvar> binds today's local date as yyyymmdd, so a scenario
			// can name the archived directory `<WS>/archived/<TODAY>-<repo>_<slug>`.
			"today": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: today <envvar>")
				}
				ts.Setenv(args[0], time.Now().Format("20060102"))
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
