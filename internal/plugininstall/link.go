package plugininstall

import (
	"os"
	"path/filepath"
)

// linkSource materializes stagingDir/source as a directory symlink (junction
// on Windows, via os.Symlink) pointing at the original local path, instead of
// copying its content. registry.Component.EntrypointPath and every existing
// invocation call site resolve through <pluginsDir>/<alias>/source/
// <entrypoint> unchanged — a symlink there is transparent to os.Stat and
// exec.Command, so --link needs no other code change anywhere (research R2).
// Creating a directory symlink on Windows requires Developer Mode or an
// elevated process; that is an accepted constraint of this development-only
// flag, not a portability regression of the plain install paths.
func linkSource(stagingDir, originalPath string) error {
	return os.Symlink(originalPath, sourceDir(stagingDir))
}

func sourceDir(stagingDir string) string {
	return filepath.Join(stagingDir, "source")
}
