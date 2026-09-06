//go:build !windows

package atomicfile

import "os"

// replace renames old onto new. On POSIX os.Rename is atomic within a
// filesystem and replaces an existing target.
func replace(old, new string) error {
	return os.Rename(old, new)
}

// syncDir fsyncs a directory so a rename survives a crash on ext4/xfs.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
