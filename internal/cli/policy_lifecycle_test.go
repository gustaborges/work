package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/workhome"
)

// Every explicit policy choice settles first-use policy in the same transaction,
// including an empty replacement of an already empty list, which changes
// nothing else.
func TestExplicitPolicyChoiceMarksTheLifecycleHandledInTheSameTransaction(t *testing.T) {
	const kind = "policy-replace"
	h := workhome.At(filepath.Join(t.TempDir(), "home"))
	before, err := adminstate.Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	if before.Lifecycle.PolicyHandled {
		t.Fatal("a fresh home starts with policy handled")
	}
	if _, err = applyRepositoryEdit(context.Background(), h, repositoryEdit{Kind: kind, Values: nil}); err != nil {
		t.Fatal(err)
	}
	after, err := adminstate.Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Lifecycle.PolicyHandled || after.Lifecycle.Initialized || after.Epoch == before.Epoch {
		t.Fatalf("%s: lifecycle = %+v, epoch %d → %d", kind, *after.Lifecycle, before.Epoch, after.Epoch)
	}
	if len(after.Config.RepositoryResolution.Locators) != 0 {
		t.Fatalf("policy = %v", after.Config.RepositoryResolution.Locators)
	}
}

// A rejected policy edit must not settle the policy.
func TestRejectedPolicyEditLeavesTheLifecycleUnhandled(t *testing.T) {
	h := workhome.At(filepath.Join(t.TempDir(), "home"))
	if _, err := applyRepositoryEdit(context.Background(), h, repositoryEdit{Kind: "policy-add", Values: []string{"ghost@nowhere/locator"}}); err == nil {
		t.Fatal("an unknown Locator was accepted")
	}
	s, err := adminstate.Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	if s.Lifecycle.PolicyHandled {
		t.Fatal("a rejected edit marked the policy handled")
	}
}
