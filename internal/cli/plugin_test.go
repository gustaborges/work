package cli

import (
	"strings"
	"testing"

	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func TestPluginNoSubcommandPrintsHelpExitZero(t *testing.T) {
	out, _, code := runWork(t, "plugin")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"install", "list"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
}

func TestPluginJSONOnParentIsUsage(t *testing.T) {
	_, _, code := runWork(t, "plugin", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestPluginInstallLocalPinnedAndList(t *testing.T) {
	home := t.TempDir()
	src := fixtures.Prepare(t, "specific-starter")

	out, errb, code := runWorkHome(t, home, "plugin", "install", src)
	if code != 0 {
		t.Fatalf("install: exit = %d\nstderr: %s", code, errb)
	}
	if !strings.Contains(out, "work: installed specific-starter (local-pinned)") {
		t.Errorf("install stdout = %q", out)
	}
	if !strings.Contains(out, "specific-starter (starter)") {
		t.Errorf("install stdout missing component line: %q", out)
	}

	out, _, code = runWorkHome(t, home, "plugin", "list")
	if code != 0 {
		t.Fatalf("list: exit = %d", code)
	}
	if !strings.Contains(out, "specific-starter") || !strings.Contains(out, "local-pinned") {
		t.Errorf("list output = %q", out)
	}

	out, _, code = runWorkHome(t, home, "plugin", "list", "--json")
	if code != 0 {
		t.Fatalf("list --json: exit = %d", code)
	}
	for _, want := range []string{`"alias": "specific-starter"`, `"origin": "local-pinned"`, `"role": "starter"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json output missing %q:\n%s", want, out)
		}
	}
}

func TestPluginListEmpty(t *testing.T) {
	out, _, code := runWork(t, "plugin", "list")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output before any install, got %q", out)
	}

	out, _, code = runWork(t, "plugin", "list", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("expected [], got %q", out)
	}
}

func TestPluginInstallAliasConflictExit32(t *testing.T) {
	home := t.TempDir()
	srcA := fixtures.Prepare(t, "specific-starter")
	srcB := fixtures.Prepare(t, "colliding-starter")

	if _, _, code := runWorkHome(t, home, "plugin", "install", srcA, "--as", "x"); code != 0 {
		t.Fatalf("first install: exit = %d", code)
	}
	_, errb, code := runWorkHome(t, home, "plugin", "install", srcB, "--as", "x")
	if code != 32 {
		t.Fatalf("exit = %d, want 32\nstderr: %s", code, errb)
	}
	if !strings.Contains(errb, "plugin-alias-conflict") {
		t.Errorf("stderr missing token: %s", errb)
	}
}

func TestPluginInstallInvalidManifestExit31(t *testing.T) {
	home := t.TempDir()
	_, errb, code := runWorkHome(t, home, "plugin", "install",
		"../../tests/fixtures/plugins/invalid-manifest")
	if code != 31 {
		t.Fatalf("exit = %d, want 31\nstderr: %s", code, errb)
	}
	if !strings.Contains(errb, "plugin-invalid") {
		t.Errorf("stderr missing token: %s", errb)
	}
}

func TestPluginInstallLinkWithRemoteIsUsage(t *testing.T) {
	_, errb, code := runWork(t, "plugin", "install", "https://example.com/x.git", "--link")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", code, errb)
	}
}

func TestPluginInstallRejectsJSON(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	_, _, code := runWork(t, "plugin", "install", src, "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestPluginInstallMissingSourceIsUsage(t *testing.T) {
	_, errb, code := runWork(t, "plugin", "install")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", code, errb)
	}
	if !strings.Contains(errb, "missing SOURCE argument") {
		t.Errorf("stderr lacks the usage message: %q", errb)
	}
}
