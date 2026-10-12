package projection

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func inventory(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		info, _ := e.Info()
		out[e.Name()] = string(b) + info.ModTime().String()
	}
	return out
}
func TestLookupPathHintPure(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt", "old", "stale", "wal", "pending"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sp ace-ação%#.db")
			want := filepath.Join(dir, "in-progress", "work", "work-state.json")
			switch kind {
			case "corrupt":
				os.WriteFile(path, []byte("broken"), 0o600)
			case "missing":
			default:
				uriPath := filepath.ToSlash(path)
				if !strings.HasPrefix(uriPath, "/") {
					uriPath = "/" + uriPath
				}
				db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: uriPath}).String())
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				if _, err := db.Exec(`CREATE TABLE works (id TEXT, snapshot_path TEXT); INSERT INTO works VALUES ('id',?)`, want); err != nil {
					t.Fatal(err)
				}
				if kind == "wal" || kind == "pending" {
					if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0`); err != nil {
						t.Fatal(err)
					}
					if kind == "pending" {
						db.Exec(`INSERT INTO works VALUES ('pending',?)`, want)
					}
				} else {
					db.Close()
				}
			}
			before := inventory(t, dir)
			for range 20 {
				got := LookupPathHint(context.Background(), path, "id")
				if kind != "missing" && kind != "corrupt" && got != want {
					t.Fatalf("hint=%q want=%q", got, want)
				}
			}
			if after := inventory(t, dir); !reflect.DeepEqual(before, after) {
				t.Fatal("hint changed database or sidecars")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			start := time.Now()
			if got := LookupPathHint(ctx, path, "id"); got != "" {
				t.Fatal(got)
			}
			if time.Since(start) > time.Second {
				t.Fatal("unbounded cancellation")
			}
		})
	}
}

func TestLookupRejectsMalformedLocations(t *testing.T) {
	for _, hint := range []string{"relative/work-state.json", filepath.Join(t.TempDir(), "other.json")} {
		path := filepath.Join(t.TempDir(), "index.db")
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE works(id TEXT, snapshot_path TEXT); INSERT INTO works VALUES ('id',?)`, hint); err != nil {
			t.Fatal(err)
		}
		db.Close()
		if got := LookupPathHint(context.Background(), path, "id"); got != "" {
			t.Fatal(got)
		}
	}
}
