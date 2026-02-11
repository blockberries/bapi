package server

import (
	"testing"
)

func TestLifecycleGuard_HappyPath(t *testing.T) {
	g := NewLifecycleGuard()

	// Init → Ready (Handshake)
	g.AcquireHandshake()
	g.CompleteHandshake()

	if !g.IsReady() {
		t.Fatal("expected Ready after handshake")
	}

	// Ready → Executing → Executed (ExecuteBlock)
	g.AcquireExecute()
	g.CompleteExecute()

	// Executed → Committing → Ready (Commit)
	g.AcquireCommit()
	g.CompleteCommit()

	if !g.IsReady() {
		t.Fatal("expected Ready after commit")
	}

	// Should be able to cycle again.
	g.AcquireExecute()
	g.CompleteExecute()
	g.AcquireCommit()
	g.CompleteCommit()

	if !g.IsReady() {
		t.Fatal("expected Ready after second cycle")
	}
}

func TestLifecycleGuard_ConcurrentAfterHandshake(t *testing.T) {
	g := NewLifecycleGuard()
	g.AcquireHandshake()
	g.CompleteHandshake()

	// CheckConcurrent should not panic after handshake.
	g.CheckConcurrent()
}

func TestLifecycleGuard_ConcurrentBeforeHandshake(t *testing.T) {
	g := NewLifecycleGuard()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for concurrent call before handshake")
		}
	}()

	g.CheckConcurrent()
}

func TestLifecycleGuard_DoubleHandshake(t *testing.T) {
	g := NewLifecycleGuard()
	g.AcquireHandshake()
	g.CompleteHandshake()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for double handshake")
		}
	}()

	g.AcquireHandshake()
}

func TestLifecycleGuard_CommitWithoutExecute(t *testing.T) {
	g := NewLifecycleGuard()
	g.AcquireHandshake()
	g.CompleteHandshake()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for commit without execute")
		}
	}()

	g.AcquireCommit()
}

func TestLifecycleGuard_ExecuteWithoutReady(t *testing.T) {
	g := NewLifecycleGuard()
	g.AcquireHandshake()
	g.CompleteHandshake()
	g.AcquireExecute()
	g.CompleteExecute()

	// Now in Executed state — calling ExecuteBlock again should panic.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for execute without ready")
		}
	}()

	g.AcquireExecute()
}

func TestLifecycleGuard_FailExecute(t *testing.T) {
	g := NewLifecycleGuard()
	g.AcquireHandshake()
	g.CompleteHandshake()

	// Execute fails → should roll back to Ready.
	g.AcquireExecute()
	g.FailExecute()

	if !g.IsReady() {
		t.Fatal("expected Ready after failed execute")
	}

	// Should be able to execute again.
	g.AcquireExecute()
	g.CompleteExecute()
	g.AcquireCommit()
	g.CompleteCommit()
}

func TestLifecycleGuard_FailHandshake(t *testing.T) {
	g := NewLifecycleGuard()
	g.AcquireHandshake()
	g.FailHandshake()

	// Should be back in Init — can handshake again.
	g.AcquireHandshake()
	g.CompleteHandshake()

	if !g.IsReady() {
		t.Fatal("expected Ready after successful retry")
	}
}

func TestLifecycleGuard_State(t *testing.T) {
	g := NewLifecycleGuard()

	if g.State() != "Init" {
		t.Errorf("expected Init, got %s", g.State())
	}

	g.AcquireHandshake()
	g.CompleteHandshake()

	if g.State() != "Ready" {
		t.Errorf("expected Ready, got %s", g.State())
	}
}
