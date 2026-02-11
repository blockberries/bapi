package bapitest

import (
	"bapi"
	"bapi/server"
	"bapi/types"
	"context"
	"testing"
	"time"
)

// Harness provides a convenient test harness for application
// developers to test their BAPI implementation against the
// lifecycle state machine.
type Harness struct {
	t   *testing.T
	srv *server.Server
}

// NewHarness creates a test harness wrapping the given application.
func NewHarness(t *testing.T, app bapi.Lifecycle) *Harness {
	t.Helper()
	return &Harness{t: t, srv: server.New(app)}
}

// Server returns the underlying server for direct access.
func (h *Harness) Server() *server.Server {
	return h.srv
}

// Genesis performs a genesis handshake with the given genesis doc.
func (h *Harness) Genesis(genesis types.GenesisDoc) types.HandshakeResponse {
	h.t.Helper()
	resp, err := h.srv.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &genesis,
	})
	if err != nil {
		h.t.Fatalf("Handshake (genesis) failed: %v", err)
	}
	return resp
}

// GenesisDefault performs a genesis handshake with a default
// genesis document.
func (h *Harness) GenesisDefault() types.HandshakeResponse {
	h.t.Helper()
	return h.Genesis(DefaultGenesis())
}

// Restart performs a restart handshake at the given block.
func (h *Harness) Restart(block types.BlockID) types.HandshakeResponse {
	h.t.Helper()
	resp, err := h.srv.Handshake(context.Background(), types.HandshakeRequest{
		LastCommitted: &block,
	})
	if err != nil {
		h.t.Fatalf("Handshake (restart) failed: %v", err)
	}
	return resp
}

// ExecuteBlock executes a block without committing.
func (h *Harness) ExecuteBlock(block types.FinalizedBlock) types.BlockOutcome {
	h.t.Helper()
	outcome, err := h.srv.ExecuteBlock(context.Background(), block)
	if err != nil {
		h.t.Fatalf("ExecuteBlock (height=%d) failed: %v", block.Height, err)
	}
	return outcome
}

// Commit commits the last executed block.
func (h *Harness) Commit() types.CommitResult {
	h.t.Helper()
	result, err := h.srv.Commit(context.Background())
	if err != nil {
		h.t.Fatalf("Commit failed: %v", err)
	}
	return result
}

// ExecuteAndCommit is a convenience that executes a block and
// commits, returning the block outcome.
func (h *Harness) ExecuteAndCommit(block types.FinalizedBlock) types.BlockOutcome {
	h.t.Helper()
	outcome := h.ExecuteBlock(block)
	h.Commit()
	return outcome
}

// CheckTx submits a transaction for mempool gate-checking.
func (h *Harness) CheckTx(tx types.Tx) types.GateVerdict {
	h.t.Helper()
	verdict, err := h.srv.CheckTx(context.Background(), tx, types.MempoolFirstSeen)
	if err != nil {
		h.t.Fatalf("CheckTx failed: %v", err)
	}
	return verdict
}

// RecheckTx re-validates a previously admitted transaction.
func (h *Harness) RecheckTx(tx types.Tx) types.GateVerdict {
	h.t.Helper()
	verdict, err := h.srv.CheckTx(context.Background(), tx, types.MempoolRevalidation)
	if err != nil {
		h.t.Fatalf("RecheckTx failed: %v", err)
	}
	return verdict
}

// Query reads application state at the latest height.
func (h *Harness) Query(path types.QueryPath, data []byte) types.StateQueryResult {
	h.t.Helper()
	result, err := h.srv.Query(context.Background(), types.StateQuery{
		Path: path,
		Data: data,
	})
	if err != nil {
		h.t.Fatalf("Query failed: %v", err)
	}
	return result
}

// MustAcceptTx asserts that a transaction is accepted.
func (h *Harness) MustAcceptTx(tx types.Tx) {
	h.t.Helper()
	v := h.CheckTx(tx)
	if !v.Accepted() {
		h.t.Fatalf("expected tx accepted, got code=%d info=%q", v.Code, v.Info)
	}
}

// MustRejectTx asserts that a transaction is rejected.
func (h *Harness) MustRejectTx(tx types.Tx) {
	h.t.Helper()
	v := h.CheckTx(tx)
	if v.Accepted() {
		h.t.Fatal("expected tx rejected, got accepted")
	}
}

// --- Helper Factories ---

// DefaultGenesis returns a minimal genesis document suitable
// for testing.
func DefaultGenesis() types.GenesisDoc {
	return types.GenesisDoc{
		ChainID:       "test-chain",
		GenesisTime:   types.TimeToTimestamp(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		InitialHeight: 1,
		ConsensusParams: types.ConsensusParams{
			MaxBlockBytes:  1024 * 1024, // 1 MiB
			MaxTxBytes:     64 * 1024,   // 64 KiB
			MaxEvidenceAge: types.DurationFromGo(24 * time.Hour),
		},
	}
}

// MakeBlock creates a FinalizedBlock at the given height with
// the provided transactions.
func MakeBlock(height uint64, txs ...types.Tx) types.FinalizedBlock {
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(height) * 5 * time.Second)
	return types.FinalizedBlock{
		Height: height,
		Time:   types.TimeToTimestamp(t),
		Txs:    txs,
	}
}

// MakeEmptyBlock creates an empty FinalizedBlock at the given height.
func MakeEmptyBlock(height uint64) types.FinalizedBlock {
	return MakeBlock(height)
}
