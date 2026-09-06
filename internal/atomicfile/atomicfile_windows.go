//go:build windows

package atomicfile

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

// replace renames old onto new using MoveFileEx with REPLACE_EXISTING so an
// existing target is overwritten, and WRITE_THROUGH so the change is flushed to
// disk before the call returns (Go's os.Rename does neither reliably on
// Windows).
//
// Windows refuses a rename while any process holds the target open — a
// concurrent writer mid-replace, a reader, or a virus scanner / search
// indexer briefly touching the file. That surfaces as ERROR_ACCESS_DENIED or
// ERROR_SHARING_VIOLATION and clears within milliseconds, so retry briefly
// before giving up.
func replace(old, new string) error {
	from, err := windows.UTF16PtrFromString(old)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(new)
	if err != nil {
		return err
	}
	const flags = windows.MOVEFILE_REPLACE_EXISTING | windows.MOVEFILE_WRITE_THROUGH

	var moveErr error
	for attempt := range 20 {
		moveErr = windows.MoveFileEx(from, to, flags)
		if moveErr == nil {
			return nil
		}
		if !errors.Is(moveErr, windows.ERROR_ACCESS_DENIED) &&
			!errors.Is(moveErr, windows.ERROR_SHARING_VIOLATION) {
			return moveErr
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}
	return moveErr
}

// syncDir is a no-op on Windows; directory handles cannot be fsynced and
// MoveFileEx already wrote through.
func syncDir(string) error { return nil }
