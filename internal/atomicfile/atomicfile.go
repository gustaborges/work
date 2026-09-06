// Package atomicfile publishes a file's contents in a single atomic step: a
// temporary file in the destination directory is written, flushed, and renamed
// over the target. A crash before the rename leaves the target untouched; a
// concurrent reader always sees either the old file or the complete new one.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile atomically replaces path with data. The destination directory must
// already exist. The resulting file has mode 0o644 (subject to umask on
// creation of the temp file).
func WriteFile(path string, data []byte) (err error) {
	dir := filepath.Dir(path)

	f, err := os.CreateTemp(dir, ".atomic-*.tmp")
	if err != nil {
		return fmt.Errorf("atomicfile: creating temp in %s: %w", dir, err)
	}
	tmp := f.Name()

	// Any failure past this point must not leave the temp file behind.
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()

	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("atomicfile: writing %s: %w", tmp, err)
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("atomicfile: syncing %s: %w", tmp, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("atomicfile: closing %s: %w", tmp, err)
	}
	if err = os.Chmod(tmp, 0o644); err != nil {
		return fmt.Errorf("atomicfile: chmod %s: %w", tmp, err)
	}
	if err = replace(tmp, path); err != nil {
		return fmt.Errorf("atomicfile: renaming %s to %s: %w", tmp, path, err)
	}
	// Best-effort durability of the rename itself. A failure here does not
	// invalidate the published contents.
	_ = syncDir(dir)
	return nil
}
