package arcp_test

import (
	"errors"
	"testing"

	"github.com/agentruntimecontrolprotocol/go-sdk"
)

func TestValidateExtensionType(t *testing.T) {
	t.Parallel()
	good := []string{
		"arcpx.example.v1",
		"arcpx.example.feature.v2",
		"arcpx.acme.workflow.beta.v3",
		"com.acme.workflow.v2",
		"io.arcp.session.close.v1",
	}
	bad := []string{
		"",
		"x-experimental",
		"x-anything",
		"session.open",       // core-looking, not namespaced
		"arcpx.example",      // missing version suffix
		"arcpx.example.vfoo", // version suffix not numeric
		"arcpx..v1",          // empty segment
		"ARCPX.example.v1",   // uppercase
		"com.v1",             // only one label before version (need >=2)
	}
	for _, t1 := range good {
		t.Run("good="+t1, func(t *testing.T) {
			t.Parallel()
			if err := arcp.ValidateExtensionType(t1); err != nil {
				t.Errorf("expected %q to be valid, got error: %v", t1, err)
			}
		})
	}
	for _, t1 := range bad {
		t.Run("bad="+t1, func(t *testing.T) {
			t.Parallel()
			err := arcp.ValidateExtensionType(t1)
			if err == nil {
				t.Errorf("expected %q to be invalid, got nil", t1)
				return
			}
			if !errors.Is(err, arcp.ErrInvalidArgument) {
				t.Errorf("expected ErrInvalidArgument, got %v", err)
			}
		})
	}
}

func TestIsCoreType(t *testing.T) {
	t.Parallel()
	core := []string{"session.open", "tool.invoke", "ping", "lease.granted"}
	notCore := []string{"arcpx.acme.v1", "com.acme.workflow.v2", ""}
	for _, c := range core {
		if !arcp.IsCoreType(c) {
			t.Errorf("expected %q to be core", c)
		}
	}
	for _, c := range notCore {
		if arcp.IsCoreType(c) {
			t.Errorf("expected %q to not be core", c)
		}
	}
}

func TestExtensionRegistry(t *testing.T) {
	t.Parallel()
	r := arcp.NewExtensionRegistry()
	if r.Has("arcpx.acme.v1") {
		t.Errorf("empty registry should not contain anything")
	}
	if err := r.Add("arcpx.acme.v1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Add("com.example.foo.v2"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !r.Has("arcpx.acme.v1") {
		t.Errorf("registry missing arcpx.acme.v1 after Add")
	}
	if r.Has("arcpx.other.v1") {
		t.Errorf("registry should not contain unregistered ns")
	}
	got := r.List()
	if len(got) != 2 {
		t.Errorf("List() = %v, want 2 items", got)
	}
	// Stable order: alphabetical.
	if got[0] != "arcpx.acme.v1" || got[1] != "com.example.foo.v2" {
		t.Errorf("unexpected List() order: %v", got)
	}
}

func TestExtensionRegistryRejectsInvalid(t *testing.T) {
	t.Parallel()
	r := arcp.NewExtensionRegistry()
	if err := r.Add("x-bad"); err == nil {
		t.Errorf("Add should reject x- prefix")
	}
	if r.Has("x-bad") {
		t.Errorf("registry should not have rejected ns")
	}
}

func TestZeroExtensionRegistry(t *testing.T) {
	t.Parallel()
	var r arcp.ExtensionRegistry
	if r.Has("anything") {
		t.Errorf("zero registry should not contain anything")
	}
	if got := r.List(); got != nil {
		t.Errorf("zero registry List() = %v, want nil", got)
	}
	// nil-safe Has check.
	var rp *arcp.ExtensionRegistry
	if rp.Has("anything") {
		t.Errorf("nil registry should not contain anything")
	}
}
