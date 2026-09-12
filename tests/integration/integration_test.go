// Package integration drives the built `work` binary end to end with
// testscript. Each .txtar file is one scenario from quickstart.md.
package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/gustaborges/work/internal/cli"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
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
			// work-set-roots <config.json> <path>... rewrites repository_roots,
			// standing in for `work repository root add` (F3) before Phase 5
			// ships that command.
			"work-set-roots": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) < 2 {
					ts.Fatalf("usage: work-set-roots <config.json> <path>...")
				}
				path := ts.MkAbs(args[0])
				cfg, err := config.Load(path)
				if err != nil {
					ts.Fatalf("load config: %v", err)
				}
				roots := make([]string, len(args)-1)
				for i, p := range args[1:] {
					roots[i] = ts.MkAbs(p)
				}
				cfg.RepositoryRoots = roots
				if err := config.Save(path, cfg); err != nil {
					ts.Fatalf("save config: %v", err)
				}
			},
			// registerlocator <alias> <name> <accepts...> registers a fake
			// repository-locator component directly in the registry —
			// standing in for `work plugin install` (F4) so F3's
			// `work repository policy`/`locator` scenarios have a second
			// locator to add/move/remove without a real plugin install.
			"registerlocator": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) < 3 {
					ts.Fatalf("usage: registerlocator <alias> <name> <accepts...>")
				}
				regPath := filepath.Join(ts.Getenv("WORK_HOME"), "state", "registry.json")
				reg, err := registry.Load(regPath)
				if err != nil {
					ts.Fatalf("registry.Load: %v", err)
				}
				reg.UpsertComponent(registry.Component{
					Alias: args[0], Name: args[1], Role: registry.RoleRepositoryLocator,
					Entrypoint: args[1], Accepts: args[2:], DisplayName: args[1],
				})
				if err := registry.Save(regPath, reg); err != nil {
					ts.Fatalf("registry.Save: %v", err)
				}
			},
			// installlocatorfixture <alias> <name> <fixture> <accepts...>
			// builds one of tests/fixtures/locators/{ok,empty,two,dupe,
			// invalid,boom} and installs it as a registered repository-locator
			// component — a runnable fake Locator for exercising the five
			// resolution outcomes (26-29) without a real plugin install.
			"installlocatorfixture": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) < 4 {
					ts.Fatalf("usage: installlocatorfixture <alias> <name> <fixture> <accepts...>")
				}
				alias, name, fixture, accepts := args[0], args[1], args[2], args[3:]
				home := ts.Getenv("WORK_HOME")
				if home == "" {
					ts.Fatalf("installlocatorfixture: $WORK_HOME is not set")
				}

				_, thisFile, _, _ := runtime.Caller(0)
				pkgDir := filepath.Join(filepath.Dir(thisFile), "..", "fixtures", "locators", fixture)
				binDir, err := os.MkdirTemp("", "locatorfixture-*")
				if err != nil {
					ts.Fatalf("%v", err)
				}
				bin := filepath.Join(binDir, fixture)
				if runtime.GOOS == "windows" {
					bin += ".exe"
				}
				if out, err := exec.Command("go", "build", "-o", bin, pkgDir).CombinedOutput(); err != nil {
					ts.Fatalf("build locator fixture %s: %v\n%s", fixture, err, out)
				}

				src := filepath.Join(home, "plugins", alias, "source")
				if err := os.MkdirAll(src, 0o755); err != nil {
					ts.Fatalf("%v", err)
				}
				destName := fixture
				if runtime.GOOS == "windows" {
					destName += ".exe"
				}
				data, err := os.ReadFile(bin)
				if err != nil {
					ts.Fatalf("%v", err)
				}
				if err := os.WriteFile(filepath.Join(src, destName), data, 0o755); err != nil {
					ts.Fatalf("%v", err)
				}

				regPath := filepath.Join(home, "state", "registry.json")
				reg, err := registry.Load(regPath)
				if err != nil {
					ts.Fatalf("registry.Load: %v", err)
				}
				reg.UpsertComponent(registry.Component{
					Alias: alias, Name: name, Role: registry.RoleRepositoryLocator,
					Entrypoint: fixture, Accepts: accepts, DisplayName: name,
				})
				if err := registry.Save(regPath, reg); err != nil {
					ts.Fatalf("registry.Save: %v", err)
				}
			},
			// setfixturematches <path>... sets LOCATOR_FIXTURE_MATCHES to the
			// proper JSON encoding of the given (MkAbs-resolved) paths. A naive
			// `env LOCATOR_FIXTURE_MATCHES=["$WORK/x"]` literal breaks on Windows,
			// where $WORK contains '\': the unescaped backslashes make the value
			// invalid JSON, so the fixture silently sees matches:[].
			"setfixturematches": func(ts *testscript.TestScript, neg bool, args []string) {
				paths := make([]string, len(args))
				for i, p := range args {
					paths[i] = ts.MkAbs(p)
				}
				b, err := json.Marshal(paths)
				if err != nil {
					ts.Fatalf("%v", err)
				}
				ts.Setenv("LOCATOR_FIXTURE_MATCHES", string(b))
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
			// snapsums <outfile> writes one sorted "<sha256>  <relpath>" line per
			// work-state.json under $WS to <outfile>, so a rebuild can be shown
			// to have modified zero snapshots (SC-006) by comparing the file
			// before and after with `cmp`.
			"snapsums": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: snapsums <outfile>")
				}
				ws := ts.Getenv("WS")
				if ws == "" {
					ts.Fatalf("snapsums: $WS is not set")
				}
				var lines []string
				err := filepath.WalkDir(ws, func(p string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if d.IsDir() || d.Name() != "work-state.json" {
						return nil
					}
					data, rerr := os.ReadFile(p)
					if rerr != nil {
						return rerr
					}
					rel, rerr := filepath.Rel(ws, p)
					if rerr != nil {
						return rerr
					}
					lines = append(lines, fmt.Sprintf("%s  %s", hex.EncodeToString(sha256Sum(data)), filepath.ToSlash(rel)))
					return nil
				})
				if err != nil {
					ts.Fatalf("snapsums: %v", err)
				}
				sort.Strings(lines)
				if err := os.WriteFile(ts.MkAbs(args[0]), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
					ts.Fatalf("snapsums: %v", err)
				}
			},
			// dborder <outfile> writes the projection's total recency order
			// ("<id> <status> <last_accessed_at>" per line, ORDER BY
			// last_accessed_at DESC, id DESC) to <outfile>, so a rebuild can be
			// shown to reproduce the classification and order byte-for-byte.
			"dborder": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: dborder <outfile>")
				}
				db := openProjection(ts)
				defer db.Close()
				rows, err := db.List()
				if err != nil {
					ts.Fatalf("dborder: %v", err)
				}
				var b strings.Builder
				for _, r := range rows {
					fmt.Fprintf(&b, "%s %s %s\n", r.ID, r.Status, r.LastAccessedAt)
				}
				if err := os.WriteFile(ts.MkAbs(args[0]), []byte(b.String()), 0o644); err != nil {
					ts.Fatalf("dborder: %v", err)
				}
			},
			// dbuserversion <n> asserts the projection's PRAGMA user_version.
			"dbuserversion": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: dbuserversion <n>")
				}
				want, err := strconv.Atoi(args[0])
				if err != nil {
					ts.Fatalf("dbuserversion: %v", err)
				}
				db := openProjection(ts)
				defer db.Close()
				got, err := db.UserVersion()
				if err != nil {
					ts.Fatalf("dbuserversion: %v", err)
				}
				if (got != want) != neg {
					ts.Fatalf("user_version = %d, want %d", got, want)
				}
			},
			// dbcount <n> asserts the number of rows in the works table.
			"dbcount": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: dbcount <n>")
				}
				want, err := strconv.Atoi(args[0])
				if err != nil {
					ts.Fatalf("dbcount: %v", err)
				}
				db := openProjection(ts)
				defer db.Close()
				rows, err := db.List()
				if err != nil {
					ts.Fatalf("dbcount: %v", err)
				}
				if (len(rows) != want) != neg {
					ts.Fatalf("works rows = %d, want %d", len(rows), want)
				}
			},
			// dbhasrow <id> asserts a works row with that id exists (negate for
			// "must not exist").
			"dbhasrow": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: dbhasrow <id>")
				}
				db := openProjection(ts)
				defer db.Close()
				_, ok, err := db.Get(args[0])
				if err != nil {
					ts.Fatalf("dbhasrow: %v", err)
				}
				if ok == neg {
					ts.Fatalf("row %s present=%v, want present=%v", args[0], ok, !neg)
				}
			},
			// dbstale <id> <ts> forces a row's last_accessed_at to <ts>, standing
			// in for an index that has drifted from its snapshot.
			"dbstale": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 2 {
					ts.Fatalf("usage: dbstale <id> <timestamp>")
				}
				db := openProjection(ts)
				defer db.Close()
				if err := db.SetAccessed(args[0], args[1]); err != nil {
					ts.Fatalf("dbstale: %v", err)
				}
			},
			// dbdroprow <id> deletes a row, standing in for a projection that has
			// lost a Work still present on disk.
			"dbdroprow": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: dbdroprow <id>")
				}
				db := openProjection(ts)
				defer db.Close()
				if err := db.Delete(args[0]); err != nil {
					ts.Fatalf("dbdroprow: %v", err)
				}
			},
			// dbghost <id> inserts a row whose snapshot does not exist on disk, so
			// a reconcile has an orphan to drop.
			"dbghost": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: dbghost <id>")
				}
				ws := ts.Getenv("WS")
				if ws == "" {
					ts.Fatalf("dbghost: $WS is not set")
				}
				db := openProjection(ts)
				defer db.Close()
				ghostDir := filepath.Join(ws, "in-progress", "ghost_ghost")
				if err := db.Upsert(projection.Work{
					ID: args[0], Slug: "ghost", Status: "in-progress", StartMode: "new",
					Starter: "local-path-starter", Branch: "ghost", BaseBranch: "main",
					BranchConvention: "freeform", RepoName: "ghost", DirPath: ghostDir,
					WorktreePath: filepath.Join(ghostDir, "worktree"),
					SnapshotPath: filepath.Join(ghostDir, "work-state.json"),
					CreatedAt:    "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-01T00:00:00Z",
				}); err != nil {
					ts.Fatalf("dbghost: %v", err)
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

func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// openProjection opens the projection database for the running scenario. The
// caller closes it.
func openProjection(ts *testscript.TestScript) *projection.DB {
	home := ts.Getenv("WORK_HOME")
	if home == "" {
		ts.Fatalf("open projection: $WORK_HOME is not set")
	}
	dbPath := filepath.Join(home, "state", "work.db")
	if _, err := os.Stat(dbPath); err != nil {
		ts.Fatalf("open projection: %v (did the scenario run a work command first?)", err)
	}
	db, err := projection.Open(dbPath)
	if err != nil {
		ts.Fatalf("open projection: %v", err)
	}
	return db
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
