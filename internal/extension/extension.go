// Package extension runs the Linkers and Importers a plugin declared for the
// core event that follows `work start`. It decides statically which of them
// are eligible, runs them one at a time in a fixed order, persists what they
// produce, and turns every failure into a warning: an extension can never
// fail, roll back or damage the Work it was started for.
//
// The package never prints. Progress and warnings are handed to an Observer,
// which the CLI border renders.
package extension

import (
	"context"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

// StartFinalized is the core event published once a Work, its worktree, its
// snapshot and its index row are all committed.
const StartFinalized = plugin.EventStartFinalized

// Operations named in events and warnings.
const (
	OpDiscover = "discover"
	OpImport   = "import"
)

// Subject is the Work an event is about, as far as eligibility and input
// resolution are concerned.
type Subject struct {
	// State is the Work's current snapshot content.
	State *work.State
	// WorktreePath is the absolute path of the Work's worktree.
	WorktreePath string
	// Starter is the component that produced the Work.
	Starter registry.Component
}

// Decision is one eligible component with the inputs it will be sent.
type Decision struct {
	Component registry.Component
	// Inputs holds only the declared inputs that resolved, keyed by bare key.
	Inputs map[string]any
}

// Indexer records where a link came from. *projection.DB satisfies it.
type Indexer interface {
	RecordProvenance(entries ...projection.Provenance) error
}

// EventKind classifies an Event.
type EventKind int

// The events an Observer sees, in the order a component produces them:
// Running first, then at most one of Linked, Imported or Warned.
const (
	Running EventKind = iota
	Linked
	Imported
	Warned
)

// Event reports progress of one component to an Observer.
type Event struct {
	Kind      EventKind
	Component string // "<alias>/<name>"
	Operation string // OpDiscover or OpImport
	// Key is the link key a Linker stored (Linked only). Never its value.
	Key string
	// Count is how many files and directories an Importer added (Imported only).
	Count int
	// Warning is set for Warned.
	Warning *diag.Warning
}

// Observer receives events as they happen. It is called on the goroutine that
// runs the extensions and must not block for long.
type Observer interface {
	OnEvent(Event)
}

// Context is everything Run needs about the Work that just started.
type Context struct {
	Home     workhome.Home
	Registry *registry.Registry
	// State is the snapshot as created; Run tracks later changes itself.
	State   *work.State
	Starter registry.Component
	// WorktreePath, WorkDir and SnapshotPath locate the Work on disk. WorkDir
	// is the directory that holds the worktree and the snapshot.
	WorktreePath string
	WorkDir      string
	SnapshotPath string
	// Index and Observer may be nil.
	Index    Indexer
	Observer Observer
	// Now defaults to time.Now.
	Now func() time.Time
}

// Report summarizes a Run. It never carries an error: a failure is a Warning.
type Report struct {
	// Ran counts the components that were started.
	Ran      int
	Warnings []diag.Warning
	// Diagnostics are non-fatal internal notes (for example a provenance write
	// that failed) meant for WORK_DEBUG output only.
	Diagnostics []string
}

// Failure is the debug detail carried in a Warning's Cause: how the component
// ended and the tail of its stderr. It is shown only under WORK_DEBUG.
type Failure struct {
	ExitCode int
	Stderr   string
	Err      error
}

// Error returns the underlying error's text; the stderr tail is carried
// separately in Stderr and never mixed into it.
func (f *Failure) Error() string { return f.Err.Error() }

// Unwrap returns the underlying error.
func (f *Failure) Unwrap() error { return f.Err }

// Run executes the automatic phases for a freshly created Work: eligible
// Linkers first, each value persisted as a link, then the Importers made
// eligible by the links that resulted. It returns after the last phase, or
// right after an interrupt. Run never returns an error and never panics on an
// extension's behavior; ctx cancellation kills the running component.
func Run(ctx context.Context, c Context) Report {
	r := newRunner(c)
	if !r.linkerPhase(ctx) {
		return r.report
	}
	r.importerPhase(ctx)
	return r.report
}

var _ Indexer = (*projection.DB)(nil)
