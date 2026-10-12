package extension

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/semconv"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workcontext"
	"github.com/gustaborges/work/internal/workhome"
)

// PublishParams captures the selected provider and Work before collection.
// Reload runs under the Work lock; Record is a best-effort postcommit trailer.
type PublishParams struct {
	Home      workhome.Home
	Expected  workcontext.Context
	Component registry.Component
	Operation string
	Value     string
	Reload    func() (*registry.Registry, error)
	Record    func(workcontext.Context, projection.Provenance) error
	Now       func() time.Time
	write     func(string, *work.State) error // deterministic commitment fault injection
}

// PublishResult describes a committed canonical link. Diagnostics concern
// operational provenance only and never turn a committed link into failure.
type PublishResult struct {
	State       *work.State
	Diagnostics []string
}

// PublishLink rereads the complete canonical state under the per-Work lock,
// replacing exactly one link and preserving concurrent unrelated publications.
func PublishLink(ctx context.Context, p PublishParams) (PublishResult, error) {
	var result PublishResult
	id := p.Expected.State.Work.ID
	failure := func(err error) error {
		return diag.Wrap(diag.LinkFailed, err, fmt.Sprintf("%s (%s) for Work %s: cannot save the link", p.Component.QualifiedName(), p.Operation, id)).WithHint("Check the Work directory is writable and retry.")
	}
	cancelled := func() error { return diag.Wrap(diag.Cancelled, ctx.Err(), "link cancelled") }
	if ctx.Err() != nil {
		return result, cancelled()
	}
	if err := semconv.ValidLinkValue(p.Value); err != nil {
		return result, diag.Wrap(diag.Usage, err, "supply a non-empty link value")
	}
	release, err := lockfile.AcquireContext(ctx, p.Home.WorkLockPath(id), 3*time.Second)
	if err != nil {
		if ctx.Err() != nil {
			return result, cancelled()
		}
		return result, failure(err)
	}
	defer release()
	// Revalidation happens after waiting, never by selecting a replacement.
	if p.Reload == nil {
		return result, failure(fmt.Errorf("provider catalog unavailable"))
	}
	reg, err := p.Reload()
	if err != nil {
		return result, err
	}
	if p.Operation == OpLink {
		latest, err := ResolveManual(reg, plugin.RoleLinker, p.Component.QualifiedName())
		if err != nil {
			return result, err
		}
		if latest.Key != p.Component.Key {
			return result, diag.New(diag.ManualComponentUnavailable, "the selected manual Linker's key has changed; select it again")
		}
	} else if semconv.ValidatePublished(reg.PluginNameOf(p.Component.Alias), p.Component.Key) != nil {
		return result, failure(fmt.Errorf("invalid published key"))
	}
	current, err := publicationContext(ctx, p.Expected)
	if err != nil {
		return result, err
	}
	current.State.Links[p.Component.Key] = p.Value
	if ctx.Err() != nil {
		return result, cancelled()
	}
	write := p.write
	if write == nil {
		write = func(path string, s *work.State) error { return work.WriteContext(ctx, path, s) }
	}
	if err := write(current.SnapshotPath, current.State); err != nil {
		if ctx.Err() != nil {
			return result, cancelled()
		}
		return result, failure(err)
	}
	// Rename is the commitment point: later cancellation is successful.
	result.State = current.State
	now := p.Now
	if now == nil {
		now = time.Now
	}
	if p.Record != nil {
		entry := projection.Provenance{WorkID: id, Section: "links", Key: p.Component.Key, SourceComponent: p.Component.QualifiedName(), SourceOperation: p.Operation, RecordedAt: now().UTC().Format(time.RFC3339)}
		if err := p.Record(current, entry); err != nil {
			result.Diagnostics = append(result.Diagnostics, fmt.Sprintf("%s: link provenance was not recorded: %v", p.Component.QualifiedName(), err))
		}
	}
	return result, nil
}

func publicationContext(ctx context.Context, expected workcontext.Context) (workcontext.Context, error) {
	path := filepath.Join(expected.WorkDir, "work-state.json")
	if expected.SnapshotPath != path {
		return workcontext.Context{}, diag.New(diag.SnapshotUnreadable, "the captured snapshot path has changed")
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		// The old directory is never recreated or redirected into the archive.
		workspace := filepath.Dir(filepath.Dir(expected.WorkDir))
		found, lookupErr := workcontext.ByID(ctx, workspace, expected.State.Work.ID, nil)
		if lookupErr == nil && found.State.Work.Status == work.StatusArchived {
			return workcontext.Context{}, diag.New(diag.TargetArchived, "that Work was archived before enrichment commitment")
		}
		if lookupErr != nil && diag.ExitCode(lookupErr) != diag.TargetNotFound.Code {
			return workcontext.Context{}, lookupErr
		}
		return workcontext.Context{}, diag.New(diag.TargetNotFound, "the active Work destination disappeared; enter an active Work and retry")
	}
	current, err := workcontext.Read(expected.WorkDir)
	if err != nil {
		return workcontext.Context{}, err
	}
	if current.WorkDir != expected.WorkDir || current.State.Work.ID != expected.State.Work.ID {
		return workcontext.Context{}, diag.New(diag.SnapshotUnreadable, "the Work directory or identity changed before enrichment commitment")
	}
	if current.State.Work.Status == work.StatusArchived {
		return workcontext.Context{}, diag.New(diag.TargetArchived, "that Work was archived before enrichment commitment")
	}
	if filepath.Base(filepath.Dir(current.WorkDir)) != work.StatusInProgress {
		return workcontext.Context{}, diag.New(diag.SnapshotUnreadable, "the Work is no longer in the active area")
	}
	return current, nil
}
