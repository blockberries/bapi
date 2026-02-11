package bapi

import (
	"fmt"
	"testing"
)

func TestHaltError(t *testing.T) {
	err := NewHaltError(42, "state root mismatch")
	if err.Height != 42 {
		t.Errorf("expected height 42, got %d", err.Height)
	}
	if err.Reason != "state root mismatch" {
		t.Errorf("unexpected reason: %s", err.Reason)
	}

	expected := "HALT at height 42: state root mismatch"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestIsHalt(t *testing.T) {
	haltErr := NewHaltError(10, "divergence")

	// Direct.
	h, ok := IsHalt(haltErr)
	if !ok {
		t.Fatal("expected IsHalt to return true")
	}
	if h.Height != 10 {
		t.Errorf("expected height 10, got %d", h.Height)
	}

	// Wrapped.
	wrapped := fmt.Errorf("wrapped: %w", haltErr)
	h2, ok2 := IsHalt(wrapped)
	if !ok2 {
		t.Fatal("expected IsHalt to unwrap wrapped error")
	}
	if h2.Height != 10 {
		t.Errorf("expected height 10, got %d", h2.Height)
	}

	// Non-halt error.
	_, ok3 := IsHalt(fmt.Errorf("just a regular error"))
	if ok3 {
		t.Fatal("expected IsHalt to return false for non-halt error")
	}

	// Nil.
	_, ok4 := IsHalt(nil)
	if ok4 {
		t.Fatal("expected IsHalt to return false for nil")
	}
}
