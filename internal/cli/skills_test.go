package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skills "github.com/gustaborges/work/ai-skills"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/skillinstall"
)

func runSkills(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append([]string{"skills"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestSkillsScopeAndPurity(t *testing.T) {
	for _, vendor := range []string{"claude", "codex"} {
		for _, scope := range []string{"local", "user"} {
			t.Run(vendor+"/"+scope, func(t *testing.T) {
				root := t.TempDir()
				t.Chdir(root)
				userHome := filepath.Join(root, "user")
				t.Setenv("HOME", userHome)
				t.Setenv("USERPROFILE", userHome)
				t.Setenv("CLAUDE_CONFIG_DIR", "")
				workHome := filepath.Join(root, "work-home")
				t.Setenv("WORK_HOME", workHome)
				flags := []string{"--scope=" + scope, "--vendor=" + vendor}
				status := append([]string{"status", "--json"}, flags...)
				s, err := runSkills(t, status...)
				if err != nil {
					t.Fatal(err)
				}
				var rows []skillinstall.Result
				if json.Unmarshal([]byte(s), &rows) != nil || len(rows) != len(skills.Names()) {
					t.Fatal(s)
				}
				base := root
				if scope == "user" {
					base = userHome
				}
				vendorDir := ".agents"
				otherDir := ".claude"
				if vendor == "claude" {
					vendorDir, otherDir = otherDir, vendorDir
				}
				for i, name := range skills.Names() {
					if rows[i].State != "absent" || rows[i].Vendor != vendor || rows[i].Skill != name || rows[i].Path != filepath.Join(base, vendorDir, "skills", name) {
						t.Fatal(rows[i])
					}
				}
				if _, err = os.Stat(filepath.Join(base, vendorDir)); !os.IsNotExist(err) {
					t.Fatal("status mutated target")
				}
				s, err = runSkills(t, append([]string{"install", "--yes"}, flags...)...)
				if err != nil {
					t.Fatal(err, s)
				}
				s, err = runSkills(t, status...)
				if err != nil {
					t.Fatal(err)
				}
				if json.Unmarshal([]byte(s), &rows) != nil {
					t.Fatal(s)
				}
				for _, row := range rows {
					if row.State != "current" {
						t.Fatal(row)
					}
				}
				if _, err = os.Stat(filepath.Join(base, otherDir)); !os.IsNotExist(err) {
					t.Fatal("installed unselected vendor")
				}
				if _, err = os.Stat(workHome); !os.IsNotExist(err) {
					t.Fatal("accessed Work home")
				}
			})
		}
	}
}

func TestSkillsInvalidSelectionDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	for _, args := range [][]string{
		{"--scope=local"}, {"--vendor=codex"},
		{"--scope=local", "--vendor="}, {"--scope=local", "--vendor=all"},
		{"--scope=local", "--vendor=codex", "--vendor=claude"},
		{"--scope=local", "--vendor=codex", "--vendor=codex"},
		{"--scope=global", "--vendor=codex"}, {"--scope=", "--vendor=codex"},
		{"--global", "--vendor=codex"}, {"--dir", root, "--vendor=codex"},
		{"work-cli-plugin-builder", "--scope=local", "--vendor=codex"},
	} {
		for _, op := range []string{"install", "status"} {
			out, err := runSkills(t, append([]string{op}, args...)...)
			if diag.ExitCode(err) != 2 || out != "" {
				t.Fatalf("%v: %q %v", args, out, err)
			}
		}
	}
	for _, args := range [][]string{
		{"install", "--vendor=codex", "--scope=local", "--json", "--yes"},
		{"install", "--vendor=codex", "--scope=local", "--json=false", "--yes"},
		{"install", "--vendor=codex", "--scope=local"},
		{"install", "--vendor=codex", "--scope=user", "--force"},
		{"status", "--vendor=codex", "--scope=local", "--yes"},
		{"status", "--vendor=codex", "--scope=local", "--force"},
	} {
		if _, err := runSkills(t, args...); diag.ExitCode(err) != 2 {
			t.Fatalf("%v: %v", args, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("usage or unconfirmed install created files", entries, err)
	}
}

func TestSkillsYesDoesNotAuthorizeReplacement(t *testing.T) {
	t.Chdir(t.TempDir())
	args := []string{"install", "--scope=local", "--vendor=codex", "--yes"}
	if _, err := runSkills(t, args...); err != nil {
		t.Fatal(err)
	}
	edit := filepath.Join(".agents", "skills", skills.Names()[0], "local-edit")
	if err := os.WriteFile(edit, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runSkills(t, args...); diag.ExitCode(err) != 62 {
		t.Fatal("--yes bypassed ownership", err)
	}
	if got, err := os.ReadFile(edit); err != nil || string(got) != "keep me" {
		t.Fatal("lost edit", err)
	}
	if _, err := runSkills(t, append(args, "--force")...); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(edit); !os.IsNotExist(err) {
		t.Fatal("force did not replace edit", err)
	}
}

func TestSkillsConfirmationListsCompleteCollection(t *testing.T) {
	ds := []skillinstall.Destination{
		{Vendor: "codex", Skill: "first", Path: "/repo/.agents/skills/first"},
		{Vendor: "codex", Skill: "second", Path: "/repo/.agents/skills/second"},
	}
	for _, force := range []bool{false, true} {
		preview := skillsInstallImpact(ds, skillinstall.Options{Vendor: "codex", Scope: "local"}, force)
		for _, want := range []string{"Vendor: codex", "Scope: local", ds[0].Skill, ds[0].Path, ds[1].Skill, ds[1].Path} {
			if !strings.Contains(preview, want) {
				t.Fatalf("missing %q in %s", want, preview)
			}
		}
		if strings.Contains(preview, "Local edits will be lost") != force {
			t.Fatal(preview)
		}
	}
}
