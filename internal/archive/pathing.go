package archive

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SanitizeBranch makes a branch name safe as a single path segment, using the
// same '/'→'-' rule F1 applies when it names an in-progress Work directory.
func SanitizeBranch(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

// ArchiveDir returns the directory an archived Work is moved into:
//
//	<workspaceRoot>/archived/<yyyymmdd>-<repoName>_<branchSanitized>
//
// where <yyyymmdd> is date in local time. branchSanitized is expected to already
// have had SanitizeBranch applied. When that path already exists — the same
// repository and branch archived more than once on the same day — the smallest
// free "-<n>" suffix ("-2", "-3", …) is appended so a previously archived Work is
// never overwritten (research R8). The returned path does not exist yet.
func ArchiveDir(workspaceRoot, repoName, branchSanitized string, date time.Time) string {
	root := filepath.Join(workspaceRoot, "archived")
	base := date.Format("20060102") + "-" + repoName + "_" + branchSanitized
	candidate := filepath.Join(root, base)
	if !exists(candidate) {
		return candidate
	}
	for n := 2; ; n++ {
		candidate = filepath.Join(root, base+"-"+strconv.Itoa(n))
		if !exists(candidate) {
			return candidate
		}
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
