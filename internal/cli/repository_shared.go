package cli

import (
	"context"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconfig"
	"github.com/gustaborges/work/internal/workhome"
)

func loadRepositoryContext() (workhome.Home, *config.Config, *registry.Registry, error) {
	h, s, err := readAdministration(context.Background())
	if err != nil {
		return h, nil, nil, err
	}
	return h, s.Config, s.Registry, nil
}

// repositoryEdit records one requested operation. A reviewed edit checks only
// its affected collection, allowing unrelated settings to merge under the lock.
type repositoryEdit struct {
	Kind          string
	Values        []string
	Before, After string
	Reviewed      []string
}

func applyRepositoryEdit(ctx context.Context, h workhome.Home, edit repositoryEdit) (*config.Config, error) {
	var result *config.Config
	err := adminstate.WithWrite(ctx, h, "work repository "+strings.Replace(edit.Kind, "-", " ", 1), func(s *adminstate.Session) error {
		current := s.Config.RepositoryResolution.Locators
		if strings.HasPrefix(edit.Kind, "root-") {
			current = s.Config.RepositoryRoots
		}
		if edit.Reviewed != nil && !slices.Equal(current, edit.Reviewed) {
			return diag.New(diag.AdministrationConflict, "reviewed repository settings changed; review again")
		}
		var err error
		switch edit.Kind {
		case "policy-add":
			err = repoconfig.AddPolicy(s.Config, s.Registry, edit.Values[0], edit.Before, edit.After)
		case "policy-remove":
			err = repoconfig.RemovePolicy(s.Config, edit.Values)
		case "policy-move":
			err = repoconfig.MovePolicy(s.Config, edit.Values[0], edit.Before, edit.After)
		case "policy-replace":
			err = repoconfig.ReplacePolicy(s.Config, s.Registry, edit.Values)
		case "root-add":
			err = repoconfig.AddRoots(s.Config, edit.Values)
		case "root-remove":
			err = repoconfig.RemoveRoots(s.Config, edit.Values)
		case "root-replace":
			err = repoconfig.ReplaceRoots(s.Config, edit.Values)
		default:
			return diag.New(diag.Usage, "unknown repository operation")
		}
		result = s.Config
		return err
	})
	return result, err
}
