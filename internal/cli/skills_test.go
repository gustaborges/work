package cli

import (
	"bytes"
	"encoding/json"
	"github.com/gustaborges/work/internal/diag"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillsScopeAndPurity(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "work-home")
	t.Setenv("WORK_HOME", home)
	run := func(args ...string) (string, error) {
		cmd := newRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	if _, err := run("skills", "install", "--dir", root, "--json"); diag.ExitCode(err) != 2 {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Fatal("usage mutated target")
	}
	s, err := run("skills", "status", "--dir", root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if json.Unmarshal([]byte(s), &rows) != nil || len(rows) != 2 || rows[0]["state"] != "absent" {
		t.Fatal(s)
	}
	if _, err = os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("status created Work home")
	}
	s, err = run("skills", "install", "--dir", root)
	if err != nil {
		t.Fatal(err, s)
	}
	if _, err = os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("install created Work home")
	}
	s, err = run("skills", "status", "--dir", root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal([]byte(s), &rows) != nil || rows[0]["state"] != "current" {
		t.Fatal(s)
	}
}
