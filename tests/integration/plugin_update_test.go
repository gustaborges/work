package integration

import (
	"context"
	"encoding/json"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func adminCommitSource(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"init"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
}
func TestAdminUpdateFixedLocalAndRemote(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "remote"}[remote], func(t *testing.T) {
			h := newAdminHarness(t)
			src := fixtures.Prepare(t, "admin-suite")
			source := src
			if remote {
				adminCommitSource(t, src)
				remotePath := filepath.ToSlash(src)
				if !strings.HasPrefix(remotePath, "/") {
					remotePath = "/" + remotePath
				}
				source = (&url.URL{Scheme: "file", Path: remotePath}).String()
			}
			h.run(t, h.root, 0, "plugin", "install", source, "--as", "a")
			h.run(t, h.root, 0, "plugin", "disable", "a")
			fixtures.Revise(t, src, "release-green", nil)
			if remote {
				adminCommitSource(t, src)
			}
			home := workhome.At(adminHome(h))
			before := adminTree(t, home.Root())
			for range 20 {
				out, _ := h.run(t, h.root, 0, "plugin", "update", "--check", "a", "--json")
				var data struct{ Updates []pluginadmin.UpdateResult }
				if err := json.Unmarshal([]byte(out), &data); err != nil {
					t.Fatal(err)
				}
				if len(data.Updates) != 1 || data.Updates[0].Status != "update-available" {
					t.Fatal(out)
				}
			}
			if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
				t.Fatal("check mutated home")
			}
			plan, err := pluginadmin.PrepareUpdates(context.Background(), home, []string{"a"}, false)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			reference := *plan.Results[0].CandidateReference
			digest := *plan.Results[0].CandidateDigest
			fixtures.Revise(t, src, "source-drift", nil)
			if remote {
				adminCommitSource(t, src)
			}
			// An unrelated latest-state edit must survive application.
			if err = adminstate.WithWrite(context.Background(), home, "unrelated", func(s *adminstate.Session) error { s.Config.RepositoryRoots = []string{h.root}; return nil }); err != nil {
				t.Fatal(err)
			}
			rows, err := pluginadmin.Apply(context.Background(), home, plan, false)
			if err != nil || rows[0].Status != "updated" {
				t.Fatal(rows, err)
			}
			s, err := adminstate.Read(context.Background(), home)
			if err != nil {
				t.Fatal(err)
			}
			p, _ := s.Registry.PackageByAlias("a")
			if p.Version != "release-green" || p.Reference != reference || p.ContentDigest != digest || s.Registry.PackageEnabled("a") || !reflect.DeepEqual(s.Config.RepositoryRoots, []string{h.root}) {
				t.Fatal(p, s.Config)
			}
			b, _ := os.ReadFile(filepath.Join(home.PluginsDir(), "a", "source", "plugin.json"))
			if strings.Contains(string(b), "source-drift") {
				t.Fatal("source retrieved twice")
			}
		})
	}
}
