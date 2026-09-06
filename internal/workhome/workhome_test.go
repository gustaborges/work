package workhome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WORK_HOME", dir)

	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if h.Root() != dir {
		t.Errorf("Root() = %q, want %q", h.Root(), dir)
	}
}

func TestResolveRelativeEnvIsAbsolute(t *testing.T) {
	t.Setenv("WORK_HOME", "relative/dotwork")
	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !filepath.IsAbs(h.Root()) {
		t.Errorf("Root() = %q, want absolute", h.Root())
	}
}

func TestResolveDefault(t *testing.T) {
	t.Setenv("WORK_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	h, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(home, ".work"); h.Root() != want {
		t.Errorf("Root() = %q, want %q", h.Root(), want)
	}
}

func TestAccessors(t *testing.T) {
	h := At("/base")
	cases := map[string]string{
		h.ConfigDir():           "/base/config",
		h.ConfigFile():          "/base/config/work.json",
		h.PluginsDir():          "/base/plugins",
		h.StateDir():            "/base/state",
		h.RegistryFile():        "/base/state/registry.json",
		h.DBFile():              "/base/state/work.db",
		h.LocksDir():            "/base/state/locks",
		h.LockPath("bootstrap"): "/base/state/locks/bootstrap.lock",
	}
	for got, want := range cases {
		if filepath.ToSlash(got) != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestEnsureLayout(t *testing.T) {
	h := At(filepath.Join(t.TempDir(), "dotwork"))
	if err := h.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	// Idempotent.
	if err := h.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout (2nd): %v", err)
	}
	for _, dir := range []string{h.ConfigDir(), h.PluginsDir(), h.StateDir(), h.LocksDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("stat %s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}
