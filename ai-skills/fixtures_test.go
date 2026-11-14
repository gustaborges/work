package skills

import (
	"context"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/marketplaceadmin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/workhome"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCompleteScaffold(t *testing.T) {
	tree, err := Tree("work-plugin-builder")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	sub, err := fs.Sub(tree, "templates/marketplace")
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(root, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		b, err := fs.ReadFile(sub, p)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, b, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := marketplace.OpenCapture(root, root)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if snapshot.Offer.Name != "example-team" || len(snapshot.Offer.Entries) != 1 {
		t.Fatal("unexpected offer")
	}
	for _, entry := range snapshot.Offer.Entries {
		resolved, err := snapshot.Resolve("fixture", entry.Name)
		if err != nil {
			t.Fatal(err)
		}
		_, manifest, err := plugininstall.Validate(resolved)
		resolved.Close()
		if err != nil {
			t.Fatal(err)
		}
		home := workhome.At(filepath.Join(t.TempDir(), "home"))
		ctx := context.Background()
		if _, err := marketplaceadmin.Add(ctx, home, marketplaceadmin.Spec{Kind: pluginid.KindLocal, Source: root}); err != nil {
			t.Fatal(err)
		}
		plan, err := pluginadmin.PrepareInstall(ctx, home, entry.Name, snapshot.Offer.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = plan.Apply(ctx, false); err != nil {
			plan.Close()
			t.Fatal(err)
		}
		plan.Close()
		if len(manifest.Components) != 4 {
			t.Fatal("missing role")
		}
	}
}
