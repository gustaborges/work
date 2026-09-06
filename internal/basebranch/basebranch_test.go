package basebranch

import (
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/gitx"
)

func TestListLocalOnly(t *testing.T) {
	dir := gittest.Repo(t)
	gittest.Git(t, dir, "branch", "feature")
	choices, err := List(gitx.Open(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(choices) != 2 {
		t.Fatalf("choices = %+v", choices)
	}
	for _, c := range choices {
		if c.Scope != ScopeLocal {
			t.Errorf("%s scope = %s", c.Short, c.Scope)
		}
	}
}

func TestListDropsRemoteHead(t *testing.T) {
	origin := gittest.Repo(t)
	clone := t.TempDir()
	gittest.Git(t, clone, "clone", "-q", origin, ".")
	choices, err := List(gitx.Open(clone))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range choices {
		if c.Refname == "refs/remotes/origin/HEAD" {
			t.Error("origin/HEAD should be dropped")
		}
	}
	var haveRemote bool
	for _, c := range choices {
		if c.Scope == ScopeRemoteTracking {
			haveRemote = true
		}
	}
	if !haveRemote {
		t.Errorf("expected a remote-tracking choice: %+v", choices)
	}
}

func TestResolve(t *testing.T) {
	choices := []Choice{
		{Refname: "refs/heads/main", Short: "main", Scope: ScopeLocal, ObjectShort: "aaa"},
		{Refname: "refs/remotes/origin/main", Short: "origin/main", Scope: ScopeRemoteTracking, ObjectShort: "bbb"},
		{Refname: "refs/remotes/upstream/main", Short: "upstream/main", Scope: ScopeRemoteTracking, ObjectShort: "ccc"},
	}

	t.Run("exact local", func(t *testing.T) {
		c, err := Resolve(choices[:1], "main")
		if err != nil || c.Refname != "refs/heads/main" {
			t.Fatalf("c=%+v err=%v", c, err)
		}
	})
	t.Run("qualified remote", func(t *testing.T) {
		c, err := Resolve(choices, "origin/main")
		if err != nil || c.Refname != "refs/remotes/origin/main" {
			t.Fatalf("c=%+v err=%v", c, err)
		}
	})
	t.Run("bare name prefers exact local over remote tail", func(t *testing.T) {
		c, err := Resolve(choices, "main")
		if err != nil || c.Scope != ScopeLocal {
			t.Fatalf("c=%+v err=%v", c, err)
		}
	})
	t.Run("ambiguous remote tail", func(t *testing.T) {
		_, err := Resolve(choices[1:], "main")
		if diag.Token(err) != diag.Usage.Token {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("no match", func(t *testing.T) {
		_, err := Resolve(choices, "nope")
		if diag.Token(err) != diag.Usage.Token {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("empty list", func(t *testing.T) {
		_, err := Resolve(nil, "main")
		if diag.Token(err) != diag.NoBaseBranch.Token {
			t.Fatalf("err=%v", err)
		}
	})
}
