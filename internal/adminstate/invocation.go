package adminstate

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func annotateRevisions(reg *registry.Registry) {
	for i := range reg.Components {
		c := &reg.Components[i]
		p, owned := reg.PackageByAlias(c.Alias)
		b, _ := json.Marshal(struct {
			Package registry.Package
			Owned   bool
		}{p, owned})
		c.Revision = digest(b)
	}
}

// CheckComponent revalidates the selection without locking. This is safe under
// a Work lock and prevents administration/Work lock inversion during publish.
func CheckComponent(reg *registry.Registry, selected registry.Component) error {
	for _, current := range reg.Components {
		if current.QualifiedName() == selected.QualifiedName() && reg.PackageEnabled(current.Alias) && reflect.DeepEqual(current, selected) {
			return nil
		}
	}
	return diag.New(diag.AdministrationConflict, "selected plugin changed or is unavailable; retry the command")
}
func Revalidate(ctx context.Context, h workhome.Home, selected registry.Component) (*registry.Registry, error) {
	s, err := Read(ctx, h)
	if err != nil {
		return nil, err
	}
	if err = CheckComponent(s.Registry, selected); err != nil {
		return nil, err
	}
	return s.Registry, nil
}

// WithInvocation holds only the administrative lock through the callback.
// The callback must finish before any Work or branch lock is acquired.
func WithInvocation(ctx context.Context, h workhome.Home, selected registry.Component, invoke func() error) error {
	release, err := acquire(ctx, h, lockfile.DefaultTimeout)
	if err != nil {
		return err
	}
	defer release()
	if _, err = Revalidate(ctx, h, selected); err != nil {
		return err
	}
	return invoke()
}
