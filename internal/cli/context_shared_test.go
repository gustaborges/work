package cli

import (
	"bytes"
	"context"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextGlobalsNeverRepair(t *testing.T) {
	home := filepath.Join(t.TempDir(), "missing")
	t.Setenv("WORK_HOME", home)
	if _, _, _, err := loadContextGlobals(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("created Work home")
	}
	for _, rel := range []string{"config/work.json", "state/registry.json"} {
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(home, rel)
			os.MkdirAll(filepath.Dir(path), 0o755)
			os.WriteFile(path, []byte("broken"), 0o600)
			if _, _, _, err := loadContextGlobals(); diag.ExitCode(err) != 16 {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(path)
			if string(b) != "broken" {
				t.Fatal("repaired globals")
			}
			os.Remove(path)
		})
	}
}

func TestSharedProviderReloadAndStreams(t *testing.T) {
	home := workhome.At(t.TempDir())
	off := false
	selected := registry.Component{Alias: "manual", Name: "linker", Role: "linker", Key: "github.pull_request", Manual: &plugin.Manual{DisplayName: "PR"}}
	r := &registry.Registry{Components: []registry.Component{selected}}
	if err := registry.Save(home.RegistryFile(), r); err != nil {
		t.Fatal(err)
	}
	snapshot, err := adminstate.Read(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	selected = snapshot.Registry.Components[0]
	if _, err := reloadContextProvider(home, selected); err != nil {
		t.Fatal(err)
	}
	r.Packages = []registry.Package{{Alias: "manual", Enabled: &off}}
	registry.Save(home.RegistryFile(), r)
	if _, err := reloadContextProvider(home, selected); diag.ExitCode(err) != 39 {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	var ui bytes.Buffer
	in := strings.NewReader("")
	cmd.SetIn(in)
	cmd.SetErr(&ui)
	pio := contextIO(cmd)
	if pio.In != in || pio.UI != &ui {
		t.Fatal("configured streams lost")
	}
	t.Chdir(t.TempDir())
	if _, err := requireActiveContext(context.Background()); diag.ExitCode(err) != 21 {
		t.Fatal(err)
	}
}

func TestNoManualCandidateGuidance(t *testing.T) {
	for _, role := range []string{plugin.RoleLinker, plugin.RoleImporter} {
		err := noManualCandidates(&registry.Registry{}, role)
		if diag.ExitCode(err) != 41 || !strings.Contains(err.Error(), "install or enable") {
			t.Fatal(err)
		}
	}
	r := &registry.Registry{Components: []registry.Component{{Alias: "manual", Name: "importer", Role: plugin.RoleImporter, Manual: &plugin.Manual{DisplayName: "Import"}, Inputs: []string{"link:github.issue"}}}}
	err := noManualCandidates(r, plugin.RoleImporter)
	if diag.ExitCode(err) != 41 || !strings.Contains(err.Error(), "required inputs") || !strings.Contains(err.Error(), "work link") {
		t.Fatal(err)
	}
}

func TestContextValidationPrecedesCatalog(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("WORK_HOME", home)
	registryPath := filepath.Join(home, "state", "registry.json")
	os.MkdirAll(filepath.Dir(registryPath), 0755)
	os.WriteFile(registryPath, []byte("invalid registry"), 0600)
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"link"}, 2}, {[]string{"import"}, 2},
		{[]string{"link", "", "secret"}, 2}, {[]string{"import", ""}, 2},
		{[]string{"link", "manual/linker", "secret"}, 21}, {[]string{"import", "manual/importer"}, 21},
	} {
		cmd := newRootCmd()
		var out, ui bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&ui)
		cmd.SetArgs(tc.args)
		err := cmd.Execute()
		if diag.ExitCode(err) != tc.code || out.Len() != 0 {
			t.Fatal(tc.args, err, out.String())
		}
	}
	b, _ := os.ReadFile(registryPath)
	if string(b) != "invalid registry" {
		t.Fatal("catalog repaired")
	}
}
