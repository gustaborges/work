package projection

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LookupPathHint reads only an optional location from an existing index. It
// never migrates or repairs it. Immutable mode avoids locks and sidecar writes;
// a live WAL can make this hint stale or absent, which callers must tolerate.
func LookupPathHint(ctx context.Context, path, id string) string {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	if ctx.Err() != nil {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	uriPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro&immutable=1"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return ""
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var hint string
	if err := db.QueryRowContext(ctx, `SELECT snapshot_path FROM works WHERE id = ?`, id).Scan(&hint); err != nil {
		return ""
	}
	if !filepath.IsAbs(hint) || filepath.Base(hint) != "work-state.json" {
		return ""
	}
	return hint
}
