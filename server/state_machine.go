// Package server provides the engine-side wrapper that enforces the
// BAPI lifecycle state machine and routes capability-gated calls.
package server

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// lifecycleState represents a state in the BAPI lifecycle state machine.
type lifecycleState uint32

const (
	// stateInit: Waiting for Handshake. No other calls allowed.
	stateInit lifecycleState = iota
	// stateReady: Handshake complete. Waiting for consensus to
	// decide a block. Concurrent calls allowed: CheckTx, Query,
	// Simulate. Sequential calls allowed: BuildProposal,
	// VerifyProposal (during consensus round).
	stateReady
	// stateExecuting: ExecuteBlock has been called. Waiting for it
	// to return. No new ExecuteBlock or Commit calls until complete.
	stateExecuting
	// stateExecuted: ExecuteBlock returned. Commit is the only
	// valid next sequential call.
	stateExecuted
	// stateCommitting: Commit has been called. Waiting for it
	// to return.
	stateCommitting
)

func (s lifecycleState) String() string {
	switch s {
	case stateInit:
		return "Init"
	case stateReady:
		return "Ready"
	case stateExecuting:
		return "Executing"
	case stateExecuted:
		return "Executed"
	case stateCommitting:
		return "Committing"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// LifecycleGuard enforces the lifecycle state machine.
// The engine wraps the application with this guard to ensure
// correct call ordering.
type LifecycleGuard struct {
	state atomic.Uint32
	// Mutex for sequential calls (ExecuteBlock, Commit,
	// BuildProposal, VerifyProposal).
	seqMu sync.Mutex
	// Tracks whether Handshake has completed (for concurrent
	// call gating).
	handshakeDone atomic.Bool
}

// NewLifecycleGuard creates a guard in the Init state.
func NewLifecycleGuard() *LifecycleGuard {
	g := &LifecycleGuard{}
	g.state.Store(uint32(stateInit))
	return g
}

// State returns the current lifecycle state.
func (g *LifecycleGuard) State() string {
	return lifecycleState(g.state.Load()).String()
}

// AcquireHandshake transitions Init → Ready.
// Panics if not in Init state.
func (g *LifecycleGuard) AcquireHandshake() {
	if !g.state.CompareAndSwap(uint32(stateInit), uint32(stateReady)) {
		panic(fmt.Sprintf("github.com/blockberries/bapi: Handshake called in state %s (expected Init)",
			lifecycleState(g.state.Load())))
	}
}

// CompleteHandshake marks handshake as done, enabling concurrent calls.
func (g *LifecycleGuard) CompleteHandshake() {
	g.handshakeDone.Store(true)
}

// FailHandshake rolls back state to Init if handshake fails.
func (g *LifecycleGuard) FailHandshake() {
	g.state.Store(uint32(stateInit))
}

// AcquireExecute transitions Ready → Executing.
// Blocks if another sequential operation is in progress.
// Panics if not in Ready state.
func (g *LifecycleGuard) AcquireExecute() {
	g.seqMu.Lock()
	if state := lifecycleState(g.state.Load()); state != stateReady {
		g.seqMu.Unlock()
		panic(fmt.Sprintf("github.com/blockberries/bapi: ExecuteBlock called in state %s (expected Ready)", state))
	}
	g.state.Store(uint32(stateExecuting))
}

// CompleteExecute transitions Executing → Executed.
func (g *LifecycleGuard) CompleteExecute() {
	g.state.Store(uint32(stateExecuted))
	g.seqMu.Unlock()
}

// FailExecute transitions Executing → Ready on error, allowing retry.
func (g *LifecycleGuard) FailExecute() {
	g.state.Store(uint32(stateReady))
	g.seqMu.Unlock()
}

// AcquireCommit transitions Executed → Committing.
// Panics if not in Executed state.
func (g *LifecycleGuard) AcquireCommit() {
	g.seqMu.Lock()
	if state := lifecycleState(g.state.Load()); state != stateExecuted {
		g.seqMu.Unlock()
		panic(fmt.Sprintf("github.com/blockberries/bapi: Commit called in state %s (expected Executed)", state))
	}
	g.state.Store(uint32(stateCommitting))
}

// CompleteCommit transitions Committing → Ready.
func (g *LifecycleGuard) CompleteCommit() {
	g.state.Store(uint32(stateReady))
	g.seqMu.Unlock()
}

// CheckConcurrent verifies that concurrent calls are allowed
// (any state after Handshake). Panics if handshake has not completed.
func (g *LifecycleGuard) CheckConcurrent() {
	if !g.handshakeDone.Load() {
		panic("github.com/blockberries/bapi: concurrent call before Handshake completed")
	}
}

// IsReady returns true if the guard is in the Ready state.
func (g *LifecycleGuard) IsReady() bool {
	return lifecycleState(g.state.Load()) == stateReady
}
