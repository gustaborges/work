package plugininstall

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func prepareSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name":"frozen","version":"blue","components":[],"conventions":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}
func TestAdminCandidateFrozen(t *testing.T) {
	dir := prepareSource(t)
	p, err := Prepare(context.Background(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`bad`), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(p.ContentDir, "plugin.json"))
	if err != nil || string(b) == "bad" || p.Manifest.Version != "blue" || p.Digest == "" {
		t.Fatal("candidate drifted", err)
	}
	root := p.ContentDir
	p.Close()
	p.Close()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("not cleaned")
	}
}
func TestAdminCandidateSymlinks(t *testing.T) {
	for _, kind := range []string{"escape", "absolute", "cycle", "contained", "manifest"} {
		t.Run(kind, func(t *testing.T) {
			dir := prepareSource(t)
			target := "plugin.json"
			switch kind {
			case "escape":
				target = "../outside"
			case "absolute":
				target = filepath.Join(t.TempDir(), "outside")
			case "cycle":
				target = "link"
			case "manifest":
				if err := os.Rename(filepath.Join(dir, "plugin.json"), filepath.Join(dir, "real.json")); err != nil {
					t.Fatal(err)
				}
				target = "real.json"
			}
			name := "link"
			if kind == "manifest" {
				name = "plugin.json"
			}
			if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
				if runtime.GOOS == "windows" {
					t.Skip(err)
				}
				t.Fatal(err)
			}
			p, err := Prepare(context.Background(), dir, Options{})
			if p != nil {
				defer p.Close()
			}
			good := kind == "contained" || kind == "manifest"
			if (err == nil) != good {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}

func TestAdminCandidateDeterministicDigestAndEntrypointEscape(t *testing.T) {
	source := prepareSource(t)
	if err := os.WriteFile(filepath.Join(source, "script"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	one, err := Prepare(context.Background(), source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer one.Close()
	two, err := Prepare(context.Background(), source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer two.Close()
	if one.Digest != two.Digest {
		t.Fatal("nondeterministic content digest")
	}
	if runtime.GOOS != "windows" {
		if err = os.Chmod(filepath.Join(source, "script"), 0644); err != nil {
			t.Fatal(err)
		}
		three, err := Prepare(context.Background(), source, Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer three.Close()
		if three.Digest == one.Digest {
			t.Fatal("ignored executable bit")
		}
	}
	if err = os.WriteFile(filepath.Join(source, "plugin.json"), []byte(`{"name":"frozen","version":"blue","components":[{"name":"run","role":"starter","entrypoint":"../escape","pattern":"^test"}],"conventions":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if p, err := Prepare(context.Background(), source, Options{}); err == nil {
		p.Close()
		t.Fatal("entrypoint escaped package")
	}
}

func TestPrepareAcceptsSourceThatIsASymlinkToTheDirectory(t *testing.T) {
	dir := prepareSource(t)
	lnk := filepath.Join(t.TempDir(), "lnk")
	if err := os.Symlink(dir, lnk); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	p, err := Prepare(context.Background(), lnk, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Reference != lnk || p.Identity != lnk {
		t.Fatalf("source identity must keep the path the user gave: %q %q", p.Identity, p.Reference)
	}
}

func TestPrepareLinkedDigestCoversOnlyThePackageSurface(t *testing.T) {
	dir := prepareSource(t)
	first, err := Prepare(context.Background(), dir, Options{Link: true})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	for _, noise := range []string{".git/objects/pack", "node_modules/dep/index.js"} {
		path := filepath.Join(dir, filepath.FromSlash(noise))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("unrelated"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	second, err := Prepare(context.Background(), dir, Options{Link: true})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if first.Digest == "" || first.Digest != second.Digest {
		t.Fatalf("linked digest depends on unrelated files: %q vs %q", first.Digest, second.Digest)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name":"frozen","version":"red","components":[],"conventions":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	third, err := Prepare(context.Background(), dir, Options{Link: true})
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	if third.Digest == first.Digest {
		t.Fatal("linked digest ignored a manifest change")
	}
}
