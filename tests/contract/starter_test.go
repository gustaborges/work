package contract

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestStarterContract(t *testing.T) {
	bin := seedBin(t, "starter")

	type resp struct {
		Repository struct {
			Path string `json:"path"`
		} `json:"repository"`
	}

	cases := []struct {
		name       string
		stdin      string
		wantExit0  bool
		checkStdin func(t *testing.T, r resp)
	}{
		{
			name:      "happy",
			stdin:     `{"arg":"/tmp/x/repo"}`,
			wantExit0: true,
			checkStdin: func(t *testing.T, r resp) {
				// The Starter runs filepath.Abs; "/tmp/x/repo" is already
				// absolute on POSIX but gets a drive prefix on Windows.
				want, _ := filepath.Abs("/tmp/x/repo")
				if r.Repository.Path != want {
					t.Errorf("path = %q, want %q", r.Repository.Path, want)
				}
			},
		},
		{
			name:      "relative arg becomes absolute",
			stdin:     `{"arg":"./repo"}`,
			wantExit0: true,
			checkStdin: func(t *testing.T, r resp) {
				if !filepath.IsAbs(r.Repository.Path) {
					t.Errorf("path %q is not absolute", r.Repository.Path)
				}
			},
		},
		{
			name:      "extra fields ignored",
			stdin:     `{"arg":"/r","x":1,"repository":{"foo":"bar"}}`,
			wantExit0: true,
			checkStdin: func(t *testing.T, r resp) {
				want, _ := filepath.Abs("/r")
				if r.Repository.Path != want {
					t.Errorf("path = %q, want %q", r.Repository.Path, want)
				}
			},
		},
		{name: "empty arg", stdin: `{"arg":""}`, wantExit0: false},
		{name: "whitespace arg", stdin: `{"arg":"   "}`, wantExit0: false},
		{name: "missing arg", stdin: `{}`, wantExit0: false},
		{name: "garbage stdin", stdin: `not json`, wantExit0: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := runBin(t, bin, c.stdin)
			if c.wantExit0 {
				if res.exitCode != 0 {
					t.Fatalf("exit = %d, stderr = %s", res.exitCode, res.stderr)
				}
				var r resp
				if err := json.Unmarshal(res.stdout, &r); err != nil {
					t.Fatalf("stdout not JSON: %q", res.stdout)
				}
				if c.checkStdin != nil {
					c.checkStdin(t, r)
				}
			} else {
				if res.exitCode == 0 {
					t.Fatalf("want non-zero exit, got 0; stdout = %q", res.stdout)
				}
				if len(res.stdout) != 0 {
					t.Errorf("want no stdout on failure, got %q", res.stdout)
				}
				if len(res.stderr) == 0 {
					t.Errorf("want a stderr message on failure")
				}
			}
		})
	}
}
