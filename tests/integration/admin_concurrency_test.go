package integration

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func TestAdminInvocationNative(t *testing.T) {
	h := newAdminHarness(t)
	source := fixtures.AdminVariant(t, "starter", "linker", "importer")
	h.run(t, h.root, 0, "plugin", "install", source, "--as", "native")
	home := workhome.At(adminHome(h))
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
		p, _ := s.Registry.PackageByAlias("native")
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
