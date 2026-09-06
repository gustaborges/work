//go:build windows

package atomicfile

import "golang.org/x/sys/windows"

// replace renames old onto new using MoveFileEx with REPLACE_EXISTING so an
// existing target is overwritten, and WRITE_THROUGH so the change is flushed to
// disk before the call returns (Go's os.Rename does neither reliably on
// Windows).
func replace(old, new string) error {
	from, err := windows.UTF16PtrFromString(old)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(new)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

// syncDir is a no-op on Windows; directory handles cannot be fsynced and
// MoveFileEx already wrote through.
func syncDir(string) error { return nil }
