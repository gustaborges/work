package integration

import (
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAdminConventionUnavailableAcrossClones(t *testing.T) {
	h := newAdminHarness(t)
	repo := filepath.Join(h.root, "repo")
	cmd := exec.Command("git", "init", "-b", "main", repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	cmd = exec.Command("git", "-C", repo, "remote", "add", "origin", "https://private-user:private-password@example.test/project?token=private-token#private-fragment")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	h.run(t, h.root, 0, "plugin", "install", fixtures.Prepare(t, "admin-suite"), "--as", "a")
	h.run(t, repo, 0, "convention", "set", "admin-team")
	clone := filepath.Join(h.root, "clone")
	cmd = exec.Command("git", "init", "-b", "main", clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	cmd = exec.Command("git", "-C", clone, "remote", "add", "origin", "https://private-user:private-password@example.test/project?token=private-token#private-fragment")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	h.run(t, h.root, 0, "plugin", "disable", "a")
	before := adminTree(t, adminHome(h))
	for _, dir := range []string{repo, clone} {
		for range 20 {
			out, ui := h.run(t, dir, 0, "convention", "show", "--json")
			for _, secret := range []string{"private-user", "private-password", "private-token", "private-fragment"} {
				if strings.Contains(out+ui, secret) {
					t.Fatal("identity leaked", out, ui)
				}
			}
			if !strings.Contains(out, `"available": false`) || !strings.Contains(out, "provider-unavailable") || !strings.Contains(out, "admin-team") {
				t.Fatal(out)
			}
		}
	}
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) {
		t.Fatal("show changed home")
	}
}

func TestAdminHubsRejectRedirectedStreams(t *testing.T) {
	h := newAdminHarness(t)
	before := adminTree(t, adminHome(h))
	for _, hub := range []string{"plugin", "repository", "convention"} {
		for _, args := range [][]string{{hub}, {hub, "--json"}} {
			out, ui := h.run(t, h.root, 2, args...)
			if strings.Contains(out+ui, "\x1b") {
				t.Fatal("terminal controls", out, ui)
			}
		}
	}
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) {
		t.Fatal("invalid hubs changed home")
	}
}
