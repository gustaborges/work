package pluginadmin

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func updateSource(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	writeVersion(t, dir, version, false)
	return dir
}
func writeVersion(t *testing.T, dir, version string, withdraw bool) {
	t.Helper()
	components := []any{map[string]any{"name": "clone", "role": "repository-locator", "entrypoint": "locator", "accepts": []string{"name"}}}
	if withdraw {
		components = []any{map[string]any{"name": "replacement", "role": "repository-locator", "entrypoint": "locator", "accepts": []string{"name"}}}
	}
	b, _ := json.Marshal(map[string]any{"name": "updates", "version": version, "components": components, "conventions": []any{map[string]any{"name": "team", "prefixes": []string{"{slug}"}}}})
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "locator"), []byte("fixture "+version), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "locator.exe"), []byte("fixture "+version), 0700); err != nil {
		t.Fatal(err)
	}
}
func installUpdateFixture(t *testing.T, h workhome.Home, source, alias string) {
	t.Helper()
	p, err := plugininstall.Prepare(context.Background(), source, plugininstall.Options{Alias: alias})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	err = adminstate.WithWrite(context.Background(), h, "fixture install", func(s *adminstate.Session) error {
		if err := p.ValidateCatalog(s.Registry); err != nil {
			return err
		}
		stage, err := p.Stage(h.PluginsDir())
		if err != nil {
			return err
		}
		if err = s.Replace(alias, stage); err != nil {
			return err
		}
		p.Register(s.Registry)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestAdminUpdateFrozenAndDependencies(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "z")
	installUpdateFixture(t, h, src, "a")
	if err := adminstate.WithWrite(ctx, h, "policy", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = []string{"ghost/old", "a/clone", "ghost/last"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	writeVersion(t, src, "a", true)
	plan, err := PrepareUpdates(ctx, h, []string{"a", "a"}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if len(plan.Results) != 1 || plan.Results[0].Status != "update-available" || !reflect.DeepEqual(plan.Results[0].PolicyReferencesRemoved, []string{"a/clone"}) {
		t.Fatal(plan.Results)
	}
	if _, err = Apply(ctx, h, plan, false); diag.ExitCode(err) != 52 {
		t.Fatal(err)
	}
	writeVersion(t, src, "later", false)
	rows, err := Apply(ctx, h, plan, true)
	if err != nil || rows[0].Status != "updated" {
		t.Fatal(rows, err)
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Registry.PackageByAlias("a")
	if p.Version != "a" || !reflect.DeepEqual(s.Config.RepositoryResolution.Locators, []string{"ghost/old", "ghost/last"}) {
		t.Fatal(p, s.Config)
	}
}
func TestAdminUpdateOriginsUnknownAndConflict(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	installUpdateFixture(t, h, src, "a")
	if _, err := PrepareUpdates(ctx, h, []string{"a", "missing"}, false); diag.ExitCode(err) != 2 {
		t.Fatal(err)
	}
	writeVersion(t, src, "two", false)
	plan, err := PrepareUpdates(ctx, h, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if _, err = SetEnabled(ctx, h, []string{"a"}, false); err != nil {
		t.Fatal(err)
	}
	rows, err := Apply(ctx, h, plan, true)
	if diag.ExitCode(err) != 50 || rows[0].Error.Token != "administration-conflict" {
		t.Fatal(rows, err)
	}
	if err = adminstate.WithWrite(ctx, h, "origins", func(s *adminstate.Session) error {
		s.Registry.UpsertPackage(registry.Package{Alias: "linked", Origin: registry.OriginLocalLinked})
		s.Registry.UpsertPackage(registry.Package{Alias: "seed", Origin: registry.OriginEmbedded})
		s.Registry.UpsertPackage(registry.Package{Alias: "unknown", Origin: registry.OriginLocalPinned, Source: src})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rows, err = Check(ctx, h, []string{"linked", "seed", "unknown"})
	if diag.ExitCode(err) != 50 || rows[0].Status != "source-managed" || rows[1].Status != "not-applicable" || rows[2].Status != "failed" {
		t.Fatal(rows, err)
	}
}

func TestAdminUpdateValidationAndCleanup(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	installUpdateFixture(t, h, src, "a")
	// Same version with different code is unchanged, but invalid same-version
	// manifests and namespace/runtime changes are still validated.
	if err := os.WriteFile(filepath.Join(src, "locator"), []byte("changed"), 0700); err != nil {
		t.Fatal(err)
	}
	rows, err := Check(ctx, h, nil)
	if err != nil || rows[0].Status != "unchanged" {
		t.Fatal(rows, err)
	}
	for _, kind := range []string{"invalid", "name", "runtime"} {
		writeVersion(t, src, "one", false)
		path := filepath.Join(src, "plugin.json")
		b, _ := os.ReadFile(path)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		switch kind {
		case "invalid":
			m["version"] = nil
		case "name":
			m["name"] = "changed"
		case "runtime":
			m["components"].([]any)[0].(map[string]any)["runtime"] = "missing-work-test-runtime"
		}
		b, _ = json.Marshal(m)
		_ = os.WriteFile(path, b, 0600)
		rows, err = Check(ctx, h, nil)
		if diag.ExitCode(err) != 50 || rows[0].Status != "failed" {
			t.Fatalf("%s: %+v %v", kind, rows, err)
		}
	}
	writeVersion(t, src, "two", false)
	plan, err := PrepareUpdates(ctx, h, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	root := plan.candidates[0].prepared.ContentDir
	plan.Close()
	plan.Close()
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("candidate leaked", err)
	}
}

func TestAdminUpdateRollbackAndCleanup(t *testing.T) {
	for _, point := range []string{"prepared", "pending", "move-backup:a", "move-publish:a", "json:registry", "json:config", "before-commit", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			h := workhome.At(t.TempDir())
			src := updateSource(t, "one")
			installUpdateFixture(t, h, src, "a")
			if err := adminstate.WithWrite(ctx, h, "policy", func(s *adminstate.Session) error {
				s.Config.RepositoryResolution.Locators = []string{"a/clone"}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			oldRegistry, _ := os.ReadFile(h.RegistryFile())
			oldConfig, _ := os.ReadFile(h.ConfigFile())
			oldContent, _ := os.ReadFile(filepath.Join(h.PluginsDir(), "a", "source", "plugin.json"))
			writeVersion(t, src, "two", true)
			plan, err := PrepareUpdates(ctx, h, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			hit := false
			_, err = ApplyWithOptions(ctx, h, plan, true, adminstate.WriteOptions{Checkpoint: func(p string) error {
				if p == point {
					hit = true
					return errors.New("injected")
				}
				return nil
			}})
			if !hit || err == nil {
				t.Fatalf("checkpoint %s: hit=%v err=%v", point, hit, err)
			}
			if point == "committed" || point == "stable" || point == "cleanup" {
				if diag.ExitCode(err) != 55 {
					t.Fatal(err)
				}
				if _, err = adminstate.Preflight(ctx, h); err != nil {
					t.Fatal(err)
				}
				s, err := adminstate.Read(ctx, h)
				if err != nil {
					t.Fatal(err)
				}
				p, _ := s.Registry.PackageByAlias("a")
				if p.Version != "two" || len(s.Config.RepositoryResolution.Locators) != 0 {
					t.Fatal(p, s.Config)
				}
			} else {
				reg, _ := os.ReadFile(h.RegistryFile())
				cfg, _ := os.ReadFile(h.ConfigFile())
				content, _ := os.ReadFile(filepath.Join(h.PluginsDir(), "a", "source", "plugin.json"))
				if string(reg) != string(oldRegistry) || string(cfg) != string(oldConfig) || string(content) != string(oldContent) {
					t.Fatal("failed update changed baseline")
				}
			}
		})
	}
}

func TestAdminUpdateDependencyDriftAndIndependentSharedOwners(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	for _, a := range []string{"a", "b"} {
		installUpdateFixture(t, h, src, a)
	}
	writeVersion(t, src, "two", false)
	plan, err := PrepareUpdates(ctx, h, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	rows, err := Apply(ctx, h, plan, false)
	if err != nil || rows[0].Status != "updated" || rows[1].Status != "updated" {
		t.Fatal(rows, err)
	}
	writeVersion(t, src, "three", true)
	plan, err = PrepareUpdates(ctx, h, []string{"a"}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if err = adminstate.WithWrite(ctx, h, "new dependency", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = []string{"a/clone"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rows, err = Apply(ctx, h, plan, true)
	if diag.ExitCode(err) != 50 || rows[0].Error.Token != "administration-conflict" {
		t.Fatal(rows, err)
	}
	s, _ := adminstate.Read(ctx, h)
	p, _ := s.Registry.PackageByAlias("a")
	if p.Version != "two" || len(s.Config.RepositoryResolution.Locators) != 1 {
		t.Fatal(p)
	}
}

func TestAdminUpdateCheckBaselineDrift(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	installUpdateFixture(t, h, src, "a")
	writeVersion(t, src, "two", false)
	rows, err := check(ctx, h, nil, func() {
		if _, err := SetEnabled(ctx, h, []string{"a"}, false); err != nil {
			t.Fatal(err)
		}
	})
	if rows != nil || diag.ExitCode(err) != 53 {
		t.Fatal(rows, err)
	}
}

func TestAdminUpdateSharedDeclarationBatch(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	for _, a := range []string{"a", "b"} {
		installUpdateFixture(t, h, src, a)
	}
	writeVersion(t, src, "two", false)
	path := filepath.Join(src, "plugin.json")
	b, _ := os.ReadFile(path)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["conventions"].([]any)[0].(map[string]any)["prefixes"] = []string{"team/{slug}"}
	b, _ = json.Marshal(m)
	_ = os.WriteFile(path, b, 0600)
	plan, err := PrepareUpdates(ctx, h, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if !reflect.DeepEqual(plan.ModifiedDeclarations("a"), []string{"convention team"}) {
		t.Fatal(plan.ModifiedDeclarations("a"))
	}
	rows, err := Apply(ctx, h, plan, false)
	if err != nil || rows[0].Status != "updated" || rows[1].Status != "updated" {
		t.Fatal(rows, err)
	}
}

func TestAdminUpdateUnresolvedRecoveryStopsBatch(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	for _, a := range []string{"a", "b"} {
		installUpdateFixture(t, h, src, a)
	}
	writeVersion(t, src, "two", false)
	plan, err := PrepareUpdates(ctx, h, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if err = os.WriteFile(h.AdministrationFile(), []byte(`{"schema":1,"epoch":99,"status":"pending","transaction_id":"missing-evidence"}`), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := Apply(ctx, h, plan, false)
	if diag.ExitCode(err) != 54 || len(rows) != 2 || rows[0].Status != "failed" || rows[1].Status != "not-applied" {
		t.Fatal(rows, err)
	}
}

func TestAdminUpdateConventionWithdrawalReview(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	installUpdateFixture(t, h, src, "a")
	writeVersion(t, src, "two", false)
	path := filepath.Join(src, "plugin.json")
	b, _ := os.ReadFile(path)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["conventions"] = []any{}
	b, _ = json.Marshal(m)
	_ = os.WriteFile(path, b, 0600)
	plan, err := PrepareUpdates(ctx, h, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	effects := plan.ConventionEffects("a")
	if len(effects) != 1 || !effects[0].AvailableBefore || effects[0].AvailableAfter || !reflect.DeepEqual(effects[0].OwnersBefore, []string{"a"}) {
		t.Fatal(effects)
	}
}

func TestAdminUpdateCombinedConventionImpact(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	for _, a := range []string{"a", "b"} {
		installUpdateFixture(t, h, src, a)
	}
	writeVersion(t, src, "two", false)
	path := filepath.Join(src, "plugin.json")
	b, _ := os.ReadFile(path)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["conventions"] = []any{}
	b, _ = json.Marshal(m)
	_ = os.WriteFile(path, b, 0600)
	plan, err := PrepareUpdates(ctx, h, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	first, last := plan.ConventionEffects("a"), plan.ConventionEffects("b")
	if len(first) != 1 || !first[0].AvailableAfter || len(last) != 1 || last[0].AvailableAfter {
		t.Fatal(first, last)
	}
	rows, err := Apply(ctx, h, plan, false)
	if err != nil || rows[0].Status != "updated" || rows[1].Status != "updated" {
		t.Fatal(rows, err)
	}
}
