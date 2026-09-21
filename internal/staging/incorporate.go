package staging

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// Incorporate creates the plan's directories and files. It never overwrites:
// files are created exclusively, and an existing directory is merged into.
// It is all or nothing — on any error it removes exactly what this call
// created (files, then directories, in reverse) before returning, so the
// error means the Work is exactly as it was.
//
// created counts what was newly created; a directory that already existed is
// not counted. Files are copied rather than renamed so the stage may live on
// another volume.
//
// WORK_FAIL_AT=incorporate:<n> makes the item with index n fail, to exercise
// the rollback.
func (p Plan) Incorporate() (created int, err error) {
	var undo []string
	rollback := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			_ = os.Remove(undo[i])
		}
	}

	failAt := injectedFailure()
	for i, it := range p.Items {
		if i == failAt {
			rollback()
			return 0, fmt.Errorf("staging: injected failure at item %d", i)
		}
		if it.IsDir {
			mkErr := os.Mkdir(it.Dest, 0o755)
			if errors.Is(mkErr, fs.ErrExist) {
				continue // merged into an existing directory
			}
			if mkErr != nil {
				rollback()
				return 0, fmt.Errorf("staging: creating %q: %w", it.Rel, mkErr)
			}
		} else if madeFile, copyErr := copyExclusive(it); copyErr != nil {
			if madeFile {
				undo = append(undo, it.Dest) // half-written, and ours to remove
			}
			rollback()
			return 0, fmt.Errorf("staging: creating %q: %w", it.Rel, copyErr)
		}
		undo = append(undo, it.Dest)
		created++
	}
	return created, nil
}

// copyExclusive creates it.Dest with O_EXCL and copies the staged file into
// it. O_EXCL is also the backstop on a case-insensitive volume, where two
// names that differ only in case reach the same file. madeFile reports
// whether this call created Dest, so a failure never claims a file it did not
// make.
func copyExclusive(it Item) (madeFile bool, err error) {
	in, err := os.Open(it.Src)
	if err != nil {
		return false, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return false, err
	}
	out, err := os.OpenFile(it.Dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return true, err
	}
	return true, out.Close()
}

// injectedFailure parses WORK_FAIL_AT=incorporate:<n>; -1 when unset.
func injectedFailure() int {
	v, ok := strings.CutPrefix(os.Getenv("WORK_FAIL_AT"), "incorporate:")
	if !ok {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return -1
	}
	return n
}
