package integration

import (
	"bufio"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplaceadmin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/installfx"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

// All writers queue behind a pending config/memory publication. Their narrow
// edits must survive in the same catalog without deadlock or stale saves.
func TestAdminConcurrentLifecycleWriters(t *testing.T) {
	h := newAdminHarness(t)
	home := workhome.At(adminHome(h))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	providers := map[string]pluginid.Provider{}
	for _, name := range []string{"a", "b", "c"} {
		source := fixtures.Prepare(t, "admin-suite")
		// Separate convention ownership makes these independent reviewed edits.
		fixtures.Revise(t, source, "release-blue", func(m map[string]any) { m["conventions"] = []any{} })
		providers[name] = installfx.InstallAs(t, home, name, source, true)
	}
	remove, err := pluginadmin.PlanUninstall(ctx, home, []pluginid.Provider{providers["b"]})
	if err != nil {
		t.Fatal(err)
	}
	add, err := marketplaceadmin.PrepareAdd(ctx, home, marketplaceadmin.Spec{Kind: pluginid.KindLocal, Source: marketplaces.Copy(t, "single")})
	if err != nil {
		t.Fatal(err)
	}
	defer add.Close()
	pending, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	result := make(chan error, 6)
	go func() {
		result <- adminstate.WithWriteOptions(ctx, home, "work repository root replace", func(s *adminstate.Session) error {
			s.Config.RepositoryRoots = []string{h.root}
			s.Config.Workspace = filepath.Join(h.root, "workspace")
			s.Memory.Set("repository-one", repoconv.NewReference(installfx.RegistrationID("a"), pluginid.Ref{Provider: providers["a"], Name: "freeform"}))
			return nil
		}, adminstate.WriteOptions{Checkpoint: func(point string) error {
			if point == "pending" {
				close(pending)
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		}})
	}()
	select {
	case <-pending:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if _, err := adminstate.Read(ctx, home); diag.ExitCode(err) != 54 {
		t.Fatal("pending reader accepted mixed state", err)
	}
	writers := []func() error{
		func() error {
			_, err := pluginadmin.SetEnabled(ctx, home, []pluginid.Provider{providers["c"]}, false)
			return err
		},
		func() error { return bootstrap.EnsureSeed(home) },
		func() error { _, err := add.Apply(ctx); return err },
		func() error { _, err := pluginadmin.Uninstall(ctx, home, remove, false); return err },
	}
	started := make(chan struct{}, len(writers))
	for _, write := range writers {
		go func() { started <- struct{}{}; result <- write() }()
	}
	for range writers {
		<-started
	}
	close(release)
	for range len(writers) + 1 {
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("administrative writers deadlocked", ctx.Err())
		}
	}
	s, err := adminstate.Read(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	_, a := s.Registry.PackageByProvider(providers["a"])
	_, b := s.Registry.PackageByProvider(providers["b"])
	remembered, _ := s.Memory.Get("repository-one")
	if !a || b || s.Registry.PackageEnabled(providers["c"]) || !s.Registry.PackageEnabled(bootstrap.Provider) ||
		s.Config.Workspace != filepath.Join(h.root, "workspace") || len(s.Config.RepositoryRoots) != 1 || remembered.Name != "freeform" ||
		len(s.Config.Marketplaces) != 1 || len(s.Catalogs.Catalogs) != 1 {
		t.Fatalf("lost concurrent edit: %+v %+v %+v", s.Registry, s.Config, s.Memory)
	}
	// Policy follows lifecycle: first setup seeded it exactly once alongside the other edits.
	if n := len(s.Config.RepositoryResolution.Locators); n != 1 {
		t.Fatalf("policy = %v", s.Config.RepositoryResolution.Locators)
	}
}

func TestAdminInvocationNative(t *testing.T) {
	h := newAdminHarness(t)
	source := fixtures.AdminVariant(t, "starter", "linker", "importer")
	home := workhome.At(adminHome(h))
	native := installfx.InstallAs(t, home, "native", source, true)
	ctx := context.Background()
	snapshot, err := adminstate.Read(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	var selectedIndex int
	for i, c := range snapshot.Registry.Components {
		if c.Name == "starter" {
			selectedIndex = i
		}
	}
	selected := snapshot.Registry.Components[selectedIndex]
	cmd := exec.Command(selected.EntrypointPath(home.PluginsDir()))
	cmd.Env = append(h.env, "WORK_ADMIN_BLOCK=pipe")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	finished := make(chan error, 1)
	started := make(chan error, 1)
	go func() {
		finished <- adminstate.WithInvocation(ctx, home, selected, func() error {
			err := cmd.Start()
			started <- err
			if err != nil {
				return err
			}
			return cmd.Wait()
		})
	}()
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Write([]byte("{\"arg\":\"admin-test\"}\n")); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ready:starter" {
		t.Fatalf("%s %v", line, err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err = adminstate.WithWrite(cancelled, home, "toggle", func(s *adminstate.Session) error { t.Error("writer ran during invocation"); return nil }); diag.ExitCode(err) != 20 {
		t.Fatal(err)
	}
	// Pure readers do not acquire the held administrative lock.
	if _, err = adminstate.Read(ctx, home); err != nil {
		t.Fatal(err)
	}
	if _, err = stdin.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
	if err = adminstate.WithWrite(ctx, home, "replace", func(s *adminstate.Session) error {
		p, _ := s.Registry.PackageByProvider(native)
		p.ContentDigest = "changed-bytes"
		s.Registry.UpsertPackage(p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = adminstate.WithInvocation(ctx, home, selected, func() error { t.Error("stale subprocess started"); return nil }); diag.ExitCode(err) != 53 {
		t.Fatal(err)
	}
}
