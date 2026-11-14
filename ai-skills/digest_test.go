package skills

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestDigestFraming(t *testing.T) {
	a := fstest.MapFS{"a": {Data: []byte("bc")}}
	b := fstest.MapFS{"ab": {Data: []byte("c")}}
	x, err := TreeDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	if x != "sha256:60e7226ed0aa2c64774b7302bf1b1ce943d2b22db93754b93e7261e07c322475" {
		t.Fatal("golden framing changed", x)
	}
	y, _ := TreeDigest(b)
	if x == y {
		t.Fatal("ambiguous framing")
	}
	a[".hidden"] = &fstest.MapFile{Data: []byte("x")}
	z, _ := TreeDigest(a)
	if z == x {
		t.Fatal("hidden file omitted")
	}
	delete(a, ".hidden")
	a[".work-skill.json"] = &fstest.MapFile{Data: []byte("marker")}
	z, _ = TreeDigest(a)
	if z != x {
		t.Fatal("root marker hashed")
	}
	a["a"].Mode = 0755
	z, _ = TreeDigest(a)
	if z != x {
		t.Fatal("mode hashed")
	}
}

func TestDigestUnsafeAndEmptyDirectories(t *testing.T) {
	for _, name := range []string{"con", "dir/file.", "back\\slash", "colon:file", "bad\x01name"} {
		if _, err := TreeDigest(fstest.MapFS{name: {Data: []byte("x")}}); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	a := fstest.MapFS{"a": {Data: []byte("x")}}
	want, _ := TreeDigest(a)
	a["empty"] = &fstest.MapFile{Mode: 0755 | fs.ModeDir}
	got, err := TreeDigest(a)
	if err != nil || got != want {
		t.Fatal("empty directory hashed", err)
	}
	a["unsafe"] = &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte("a")}
	if _, err := TreeDigest(a); err == nil {
		t.Fatal("symlink accepted")
	}
}
