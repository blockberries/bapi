package server

import (
	"bapi"
	"bapi/types"
	"context"
	"testing"
)

// testApp is a minimal mock that implements all BAPI interfaces
// to avoid an import cycle with bapi/testing.
type testApp struct {
	caps           types.Capabilities
	handshakeCalls int
}

var (
	_ bapi.Lifecycle       = (*testApp)(nil)
	_ bapi.ProposalControl = (*testApp)(nil)
	_ bapi.VoteExtender    = (*testApp)(nil)
	_ bapi.StateSync       = (*testApp)(nil)
	_ bapi.Simulator       = (*testApp)(nil)
)

func (a *testApp) Handshake(_ context.Context, _ types.HandshakeRequest) (types.HandshakeResponse, error) {
	a.handshakeCalls++
	return types.HandshakeResponse{Capabilities: a.caps}, nil
}

func (a *testApp) CheckTx(_ context.Context, _ types.Tx, _ types.MempoolContext) (types.GateVerdict, error) {
	return types.GateVerdict{Code: 0}, nil
}

func (a *testApp) ExecuteBlock(_ context.Context, block types.FinalizedBlock) (types.BlockOutcome, error) {
	outcomes := make([]types.TxOutcome, len(block.Txs))
	for i := range block.Txs {
		outcomes[i] = types.TxOutcome{Index: uint32(i), Code: 0}
	}
	return types.BlockOutcome{
		TxOutcomes: outcomes,
		AppHash:    types.AppHash{0x01},
	}, nil
}

func (a *testApp) Commit(_ context.Context) (types.CommitResult, error) {
	return types.CommitResult{}, nil
}

func (a *testApp) Query(_ context.Context, _ types.StateQuery) (types.StateQueryResult, error) {
	return types.StateQueryResult{}, nil
}

func (a *testApp) BuildProposal(_ context.Context, pctx types.ProposalContext) (types.BuiltProposal, error) {
	return types.BuiltProposal{Txs: pctx.MempoolTxs}, nil
}

func (a *testApp) VerifyProposal(_ context.Context, _ types.ReceivedProposal) (types.ProposalVerdict, error) {
	return types.ProposalVerdict{Accept: true}, nil
}

func (a *testApp) ExtendVote(_ context.Context, _ types.VoteContext) ([]byte, error) {
	return nil, nil
}

func (a *testApp) VerifyExtension(_ context.Context, _ types.ReceivedExtension) (types.ExtensionVerdict, error) {
	return types.ExtensionAccept, nil
}

func (a *testApp) AvailableSnapshots(_ context.Context) ([]types.SnapshotDescriptor, error) {
	return nil, nil
}

func (a *testApp) ExportSnapshot(_ context.Context, height uint64, format uint32) (<-chan types.SnapshotChunk, *types.SnapshotDescriptor, error) {
	ch := make(chan types.SnapshotChunk)
	close(ch)
	return ch, &types.SnapshotDescriptor{Height: height, Format: format}, nil
}

func (a *testApp) ImportSnapshot(_ context.Context, _ types.SnapshotDescriptor, chunks <-chan types.SnapshotChunk) (types.ImportResult, error) {
	for range chunks {
	}
	ah := types.AppHash{0x01}
	return types.ImportResult{Status: types.ImportOK, AppHash: &ah}, nil
}

func (a *testApp) Simulate(_ context.Context, _ types.Tx) (types.TxOutcome, error) {
	return types.TxOutcome{Code: 0}, nil
}

// --- Tests ---

func TestServer_Handshake_Genesis(t *testing.T) {
	app := &testApp{}
	srv := New(app)

	resp, err := srv.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &types.GenesisDoc{ChainID: "test"},
	})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	if resp.Capabilities != 0 {
		t.Errorf("expected no capabilities, got %s", resp.Capabilities)
	}
	if app.handshakeCalls != 1 {
		t.Errorf("expected 1 handshake call, got %d", app.handshakeCalls)
	}
}

func TestServer_Handshake_WithCapabilities(t *testing.T) {
	app := &testApp{
		caps: types.CapProposalControl | types.CapSimulation,
	}
	srv := New(app)

	resp, err := srv.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &types.GenesisDoc{ChainID: "test"},
	})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	if !resp.Capabilities.Has(types.CapProposalControl) {
		t.Error("expected CapProposalControl")
	}
	if !resp.Capabilities.Has(types.CapSimulation) {
		t.Error("expected CapSimulation")
	}
}

func TestServer_ExecuteCommitCycle(t *testing.T) {
	app := &testApp{}
	srv := New(app)

	_, err := srv.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &types.GenesisDoc{ChainID: "test"},
	})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}

	block := types.FinalizedBlock{
		Height: 1,
		Txs:    []types.Tx{{0x01}},
	}

	outcome, err := srv.ExecuteBlock(context.Background(), block)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(outcome.TxOutcomes) != 1 {
		t.Errorf("expected 1 tx outcome, got %d", len(outcome.TxOutcomes))
	}

	if srv.LastOutcome() == nil {
		t.Error("expected non-nil LastOutcome between execute and commit")
	}

	_, err = srv.Commit(context.Background())
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	if srv.LastOutcome() != nil {
		t.Error("expected nil LastOutcome after commit")
	}
}

func TestServer_CheckTxConcurrent(t *testing.T) {
	app := &testApp{}
	srv := New(app)

	_, err := srv.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &types.GenesisDoc{ChainID: "test"},
	})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, err := srv.CheckTx(context.Background(), types.Tx{0x01}, types.MempoolFirstSeen)
			if err != nil {
				t.Errorf("CheckTx error: %v", err)
			}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestServer_CapabilityGating(t *testing.T) {
	app := &testApp{}
	srv := New(app)

	_, err := srv.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &types.GenesisDoc{ChainID: "test"},
	})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}

	if srv.AsProposalControl() != nil {
		t.Error("expected nil ProposalControl when not declared")
	}
	if srv.AsVoteExtender() != nil {
		t.Error("expected nil VoteExtender when not declared")
	}
	if srv.AsStateSync() != nil {
		t.Error("expected nil StateSync when not declared")
	}
	if srv.AsSimulator() != nil {
		t.Error("expected nil Simulator when not declared")
	}
}

func TestServer_CapabilityFullAccess(t *testing.T) {
	app := &testApp{
		caps: types.CapProposalControl | types.CapVoteExtensions |
			types.CapStateSync | types.CapSimulation,
	}
	srv := New(app)

	_, err := srv.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &types.GenesisDoc{ChainID: "test"},
	})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}

	if srv.AsProposalControl() == nil {
		t.Error("expected non-nil ProposalControl")
	}
	if srv.AsVoteExtender() == nil {
		t.Error("expected non-nil VoteExtender")
	}
	if srv.AsStateSync() == nil {
		t.Error("expected non-nil StateSync")
	}
	if srv.AsSimulator() == nil {
		t.Error("expected non-nil Simulator")
	}
}
