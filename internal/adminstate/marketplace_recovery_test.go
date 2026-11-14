package adminstate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/workhome"
)

type crash struct{ point string }

// crashAt runs op and aborts at point with a panic that bypasses compensation
// the way a killed process does: no rollback or cleanup code after the
// checkpoint executes, and only the deferred lock release runs.
func crashAt(t *testing.T, h workhome.Home, point string, fn func(*Session) error) {
	t.Helper()
	defer func() {
		switch r := recover().(type) {
		case nil:
			t.Fatalf("operation completed without reaching %q", point)
		case crash:
		default:
			panic(r)
		}
	}()
	_ = withWrite(context.Background(), h, "work plugin marketplace upgrade team", fn, func(p string) error {
		if p == point {
			panic(crash{p})
		}
		return nil
	})
}

// A source removal in miniature: replace one package, remove the last package
// of a namespace, rewrite config and the catalog image together.
func sourceRemoval(t *testing.T, h workhome.Home) func(*Session) error {
	return func(s *Session) error {
		if err := s.Replace(prov("keep", "other"), stageWith(t, s, "new", "after")); err != nil {
			return err
		}
		if err := s.Remove(prov("gone", "team")); err != nil {
			return err
		}
		s.Config.Workspace = "after"
		s.Catalogs.Catalogs = s.Catalogs.Catalogs[:0]
		s.Persist("catalogs")
		return nil
	}
}

func TestCatalogImageAndNestedMovesRecoverAtEveryCheckpoint(t *testing.T) {
	points := []string{
		"prepared", "pending", "move-backup:keep@other", "move-publish:keep@other", "move-backup:gone@team", "move-publish:gone@team",
		"json:config", "json:catalogs", "before-commit", "committed", "stable", "cleanup",
	}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			keep := seedRoot(t, h, prov("keep", "other"), "old", "before")
			gone := seedRoot(t, h, prov("gone", "team"), "old", "before")
			if err := WithWrite(context.Background(), h, "seed", func(s *Session) error {
				s.Config.Workspace = "before"
				s.Catalogs.Upsert(storedLocal(t))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			crashAt(t, h, point, sourceRemoval(t, h))

			// Before the pending epoch is published nothing demands recovery; the
			// journal is then only evidence. From pending until stable it does.
			pending := point != "prepared" && point != "stable" && point != "cleanup"
			if _, err := Read(context.Background(), h); pending && diag.ExitCode(err) != 54 {
				t.Fatalf("a read of pending state must demand recovery, got %v", err)
			}
			if _, err := Preflight(context.Background(), h); err != nil {
				t.Fatal(err)
			}
			committed := point == "committed" || point == "stable" || point == "cleanup"
			s, err := Read(context.Background(), h)
			if err != nil {
				t.Fatal(err)
			}
			if (s.Config.Workspace == "after") != committed || (len(s.Catalogs.Catalogs) == 0) != committed {
				t.Fatalf("partial JSON set: workspace=%q catalogs=%d", s.Config.Workspace, len(s.Catalogs.Catalogs))
			}
			if b, _ := os.ReadFile(filepath.Join(keep, "new")); committed != (string(b) == "after") {
				t.Fatalf("replacement committed=%v but new=%q", committed, b)
			}
			if _, err := os.Stat(filepath.Join(gone, "old")); committed == (err == nil) {
				t.Fatalf("removal committed=%v but old present=%v", committed, err == nil)
			}
			// The emptied namespace is pruned only with the committed removal.
			if ns, _ := h.MarketplaceDir("team"); committed == !isGone(ns) {
				t.Fatalf("namespace directory pruning committed=%v gone=%v", committed, isGone(ns))
			}
			// No transaction evidence, backup or stage survives recovery.
			entries, _ := os.ReadDir(h.PluginsDir())
			for _, e := range entries {
				if e.Name() != "other" && e.Name() != "team" {
					t.Errorf("stray package evidence %q", e.Name())
				}
			}
			if rest, _ := os.ReadDir(h.AdministrationDir()); len(rest) != 0 {
				t.Errorf("journal evidence not cleaned: %v", rest)
			}
		})
	}
}

func isGone(p string) bool { _, err := os.Lstat(p); return os.IsNotExist(err) }

func TestJournalIsValidatedByTheSingleCurrentValidator(t *testing.T) {
	image := func(b string) map[string]any {
		return map[string]any{"exists": true, "bytes": []byte(b), "digest": digest([]byte(b))}
	}
	absent := map[string]any{"exists": false, "bytes": nil, "digest": ""}
	base := func() map[string]any {
		return map[string]any{
			"schema": 1, "id": "0123456789abcdef0123456789abcdef", "operation": "work plugin marketplace add x", "phase": "prepared",
			"files": []any{map[string]any{"name": "catalogs", "before": absent, "after": image("{}")}}, "moves": []any{},
		}
	}
	mutations := map[string]func(j map[string]any){
		"future schema":      func(j map[string]any) { j["schema"] = 2 },
		"missing schema":     func(j map[string]any) { delete(j, "schema") },
		"id mismatch":        func(j map[string]any) { j["id"] = "ffffffffffffffffffffffffffffffff" },
		"unknown phase":      func(j map[string]any) { j["phase"] = "half" },
		"empty operation":    func(j map[string]any) { j["operation"] = "" },
		"unknown field":      func(j map[string]any) { j["converted_from"] = "older" },
		"retired image name": func(j map[string]any) { j["files"].([]any)[0].(map[string]any)["name"] = "reference-package" },
		"arbitrary image":    func(j map[string]any) { j["files"].([]any)[0].(map[string]any)["name"] = "../../etc/passwd" },
		"duplicate image":    func(j map[string]any) { j["files"] = append(j["files"].([]any), j["files"].([]any)[0]) },
		"digest mismatch": func(j map[string]any) {
			j["files"].([]any)[0].(map[string]any)["after"].(map[string]any)["digest"] = "sha256:00"
		},
		"non-JSON image": func(j map[string]any) { j["files"].([]any)[0].(map[string]any)["after"] = image("not json") },
		"absent with bytes": func(j map[string]any) {
			j["files"].([]any)[0].(map[string]any)["before"] = map[string]any{"exists": false, "bytes": []byte("x"), "digest": ""}
		},
		"nothing to do": func(j map[string]any) { j["files"] = []any{} },
		"legacy alias move": func(j map[string]any) {
			j["moves"] = []any{map[string]any{"alias": "a", "stage": ".admin-stage-x", "after_digest": "d"}}
		},
		"address escape move": func(j map[string]any) {
			j["moves"] = []any{map[string]any{"provider": map[string]string{"marketplace": "..", "plugin": "x"}, "remove": true}}
		},
		"duplicate move": func(j map[string]any) {
			m := map[string]any{"provider": map[string]string{"marketplace": "m", "plugin": "p"}, "remove": true}
			j["moves"] = []any{m, m}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			j := base()
			mutate(j)
			id := "0123456789abcdef0123456789abcdef"
			dir, _ := h.TransactionDir(id)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(j)
			if err := os.WriteFile(filepath.Join(dir, "journal.json"), raw, 0o600); err != nil {
				t.Fatal(err)
			}
			control := Control{Schema: 1, Epoch: 3, Status: "pending", TransactionID: &id}
			if err := saveJSON(h.AdministrationFile(), control); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(h.AdministrationFile())

			if _, err := loadJournal(h, id); diag.ExitCode(err) != 54 {
				t.Fatalf("loadJournal = %v", err)
			}
			if _, err := Preflight(context.Background(), h); diag.ExitCode(err) != 54 {
				t.Fatalf("Preflight = %v", err)
			}
			after, _ := os.ReadFile(h.AdministrationFile())
			if string(before) != string(after) {
				t.Fatal("unsupported evidence was rewritten")
			}
			if got, _ := os.ReadFile(filepath.Join(dir, "journal.json")); string(got) != string(raw) {
				t.Fatal("unsupported journal was converted or deleted")
			}
		})
	}
	// Control the same way: unsupported control is invalid recovery state, untouched.
	h := workhome.At(t.TempDir())
	_ = os.MkdirAll(h.StateDir(), 0o700)
	bad := []byte(`{"schema":2,"epoch":9,"status":"stable","transaction_id":null}`)
	_ = os.WriteFile(h.AdministrationFile(), bad, 0o600)
	if _, err := Preflight(context.Background(), h); diag.ExitCode(err) != 54 {
		t.Fatalf("future control = %v", err)
	}
	if got, _ := os.ReadFile(h.AdministrationFile()); string(got) != string(bad) {
		t.Fatal("future control rewritten")
	}
}

func TestPreparedAndCommittedJournalsReplayDifferently(t *testing.T) {
	for _, committed := range []bool{false, true} {
		h := workhome.At(t.TempDir())
		crashPoint := "before-commit"
		if committed {
			crashPoint = "committed"
		}
		crashAt(t, h, crashPoint, func(s *Session) error { s.Config.Workspace = "after"; return nil })
		recovered, err := Preflight(context.Background(), h)
		if err != nil || len(recovered) != 1 {
			t.Fatalf("committed=%v: %v %v", committed, recovered, err)
		}
		s, _ := Read(context.Background(), h)
		if (s.Config.Workspace == "after") != committed {
			t.Fatalf("committed=%v but workspace=%q", committed, s.Config.Workspace)
		}
	}
}

func TestNoopOperationsPreserveBytesAndEpoch(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	if err := WithWrite(ctx, h, "setup", func(s *Session) error {
		s.Config.Workspace = "w"
		s.Catalogs.Upsert(storedLocal(t))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[string]string {
		out := map[string]string{}
		for _, p := range []string{h.ConfigFile(), h.CatalogsFile(), h.AdministrationFile()} {
			b, _ := os.ReadFile(p)
			out[p] = string(b)
		}
		return out
	}
	before := snapshot()
	if err := WithWrite(ctx, h, "same", func(s *Session) error {
		s.Config.Workspace = "w"
		s.Catalogs.Upsert(storedLocal(t))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for k, v := range snapshot() {
		if before[k] != v {
			t.Errorf("%s changed on a no-op", filepath.Base(k))
		}
	}
	if rest, _ := os.ReadDir(h.AdministrationDir()); len(rest) != 0 {
		t.Error("a no-op left a journal")
	}
}
