//go:build planneddeps

// Package planneddeps pins the versions of libraries that the F1 design calls
// for but that no real code imports yet. Each blank import here is deleted as
// the owning package starts using the library directly; the file goes away
// once the list is empty.
package planneddeps

import (
	_ "charm.land/bubbletea/v2"
	_ "charm.land/huh/v2"
	_ "charm.land/lipgloss/v2"
	_ "github.com/rogpeppe/go-internal/testscript"
	_ "golang.org/x/sys/unix"
	_ "golang.org/x/term"
	_ "modernc.org/sqlite"
)
