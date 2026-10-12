package integration

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func statusInventory(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Read current metadata rather than the directory enumeration's cached
		// timestamps, which can lag behind fixture writes on Windows.
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		value := fmt.Sprint(info.Mode(), info.ModTime().UnixNano())
		if !d.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(" %x", sha256.Sum256(b))
		}
		out[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStatusPurity(t *testing.T) {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	bin := filepath.Join(t.TempDir(), "work"+suffix)
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/gustaborges/work/cmd/work").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	fixture := fixtures.Prepare(t, "manual-suite")
	spyDir := t.TempDir()
	spy, err := os.ReadFile(filepath.Join(fixture, "linker"+suffix))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spyDir, "git"+suffix), spy, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing", "corrupt", "stale", "old", "wal", "no-home", "old-root-hint", "old-root-no-hint"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			home := workhome.At(filepath.Join(root, "home"))
			ws := filepath.Join(root, "work space ação")
			dir := filepath.Join(ws, "in-progress", "topic")
			os.MkdirAll(filepath.Join(dir, "notes"), 0o755)
			s := &work.State{Work: work.WorkSection{ID: "exact-ID", Status: "in-progress", StartMode: "contribution", Starter: "test", Branch: "feature/topic", BaseBranch: "main", CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-01T00:00:00Z"}}
			path := filepath.Join(dir, "work-state.json")
			if err := work.Write(path, s); err != nil {
				t.Fatal(err)
			}
			archiveDir := filepath.Join(ws, "archived", "old-topic")
			os.MkdirAll(filepath.Join(archiveDir, "notes"), 0o755)
			archived := *s
			archived.Work.ID = "archived-ID"
			archived.Archive(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
			archivePath := filepath.Join(archiveDir, "work-state.json")
			if err := work.Write(archivePath, &archived); err != nil {
				t.Fatal(err)
			}
			// Retain an accepted old schema and prove no upgrade occurs.
			b, _ := os.ReadFile(path)
			b = bytes.Replace(b, []byte(`"schema": 3`), []byte(`"schema": 2`), 1)
			os.WriteFile(path, b, 0o644)
			if kind != "no-home" {
				cfg := config.Default()
				cfg.Workspace = ws
				if kind == "old-root-hint" || kind == "old-root-no-hint" {
					cfg.Workspace = filepath.Join(root, "new-workspace")
				}
				config.Save(home.ConfigFile(), cfg)
				os.MkdirAll(home.StateDir(), 0o755)
				// Any plugin launch or Git probe is observable through the same native spy.
				source := filepath.Join(home.PluginsDir(), "manual", "source")
				os.MkdirAll(source, 0o755)
				os.WriteFile(filepath.Join(source, "linker"+suffix), spy, 0o755)
				registry.Save(home.RegistryFile(), &registry.Registry{Components: []registry.Component{{Alias: "manual", Name: "linker", Role: "linker", Entrypoint: "linker"}}})
			}
			if kind == "corrupt" {
				os.WriteFile(home.DBFile(), []byte("corrupt index"), 0o600)
			}
			var db *sql.DB
			if kind == "stale" || kind == "old" || kind == "wal" || kind == "old-root-hint" {
				var err error
				db, err = sql.Open("sqlite", home.DBFile())
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				if _, err := db.Exec(`CREATE TABLE works(id TEXT, snapshot_path TEXT, branch TEXT); INSERT INTO works VALUES ('exact-ID',?, 'stale-branch')`, path); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`INSERT INTO works VALUES ('archived-ID',?, 'stale-archived')`, archivePath); err != nil {
					t.Fatal(err)
				}
				if kind == "wal" {
					if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; UPDATE works SET branch='pending'`); err != nil {
						t.Fatal(err)
					}
				} else {
					db.Close()
				}
			}
			handoff := filepath.Join(root, "handoff")
			os.WriteFile(handoff, []byte("untouched"), 0o600)
			before := statusInventory(t, root)
			for range 20 {
				for _, target := range []struct{ cwd, id string }{{dir, "exact-ID"}, {filepath.Join(dir, "notes"), "exact-ID"}, {root, "exact-ID"}, {archiveDir, "archived-ID"}, {filepath.Join(archiveDir, "notes"), "archived-ID"}, {root, "archived-ID"}} {
					cwd := target.cwd
					for _, machine := range []bool{false, true} {
						args := []string{"status"}
						if cwd == root {
							args = append(args, target.id)
						}
						if machine {
							args = append(args, "--json")
						}
						cmd := exec.Command(bin, args...)
						cmd.Dir = cwd
						cmd.Env = append(os.Environ(), "WORK_HOME="+home.Root(), "PATH="+spyDir, "WORK_CD_FILE="+handoff, "WORK_FIXTURE_LOG="+filepath.Join(root, "launch.log"))
						var stdout, stderr bytes.Buffer
						cmd.Stdout = &stdout
						cmd.Stderr = &stderr
						err := cmd.Run()
						unavailable := cwd == root && (kind == "no-home" || kind == "old-root-no-hint")
						if unavailable {
							if err == nil || stdout.Len() != 0 {
								t.Fatal("unexpected lookup success")
							}
						} else if err != nil || stderr.Len() != 0 {
							t.Fatalf("%v %s", err, stderr.String())
						} else if !strings.Contains(stdout.String(), "feature/topic") {
							t.Fatal("displayed index data instead of canonical state")
						}
					}
				}
			}
			if after := statusInventory(t, root); !reflect.DeepEqual(before, after) {
				for path, value := range before {
					if after[path] != value {
						t.Errorf("%s: before %q, after %q", path, value, after[path])
					}
				}
				for path := range after {
					if _, ok := before[path]; !ok {
						t.Errorf("new path: %s", path)
					}
				}
				t.Fatal("status changed application files, times or inventory")
			}
		})
	}
}
