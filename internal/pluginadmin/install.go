package pluginadmin

import (
	"context"
	"errors"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/workhome"
)

// InstallPrepared publishes the caller-owned, reviewed source through the same
// journal used by direct installation. The caller closes it after review/apply.
func InstallPrepared(ctx context.Context, h workhome.Home, p *plugininstall.Prepared) (plugininstall.Result, error) {
	var result plugininstall.Result
	err := adminstate.WithWrite(ctx, h, "work plugin install", func(s *adminstate.Session) error {
		if err := p.ValidateCatalog(s.Registry); err != nil {
			return err
		}
		stage, err := p.Stage(h.PluginsDir())
		if err != nil {
			return diag.Wrap(diag.PluginInstallFailed, err, "cannot stage plugin content")
		}
		if err = s.Replace(p.Alias, stage); err != nil {
			return err
		}
		result = p.Register(s.Registry)
		return nil
	})
	return result, installError(err, "cannot publish plugin content and registration")
}
func InstallEmbedded(ctx context.Context, h workhome.Home) (plugininstall.Result, error) {
	var result plugininstall.Result
	err := adminstate.WithWrite(ctx, h, "work plugin install --embedded", func(s *adminstate.Session) error {
		var err error
		result, err = bootstrap.InstallEmbeddedLocked(h, s)
		return err
	})
	return result, installError(err, "cannot install the embedded reference package")
}
func installError(err error, message string) error {
	if err == nil {
		return nil
	}
	var typed *diag.Error
	if errors.As(err, &typed) {
		return err
	}
	return diag.Wrap(diag.PluginInstallFailed, err, message)
}
