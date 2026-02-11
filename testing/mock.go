// Package bapitest provides test utilities for BAPI application
// development, including a configurable mock, a test harness,
// and a lifecycle compliance test suite.
package bapitest

import (
	"bapi"
	"bapi/types"
	"context"
	"sync"
	"sync/atomic"
)

// Compile-time check that MockApp satisfies all interfaces.
var (
	_ bapi.Lifecycle       = (*MockApp)(nil)
	_ bapi.ProposalControl = (*MockApp)(nil)
	_ bapi.VoteExtender    = (*MockApp)(nil)
	_ bapi.StateSync       = (*MockApp)(nil)
	_ bapi.Simulator       = (*MockApp)(nil)
)

// MockApp is a configurable mock BAPI application for engine testing.
// All methods are configurable via function fields. Unconfigured
// methods return sensible zero-value defaults.
//
// MockApp implements all optional interfaces so it can be used to
// test capability discovery. Control which capabilities are declared
// via the DeclaredCapabilities field.
type MockApp struct {
	mu sync.Mutex

	// DeclaredCapabilities controls the bitfield returned at handshake.
	DeclaredCapabilities types.Capabilities

	// Configurable handlers. If nil, defaults are used.
	HandshakeFn       func(context.Context, types.HandshakeRequest) (types.HandshakeResponse, error)
	CheckTxFn         func(context.Context, types.Tx, types.MempoolContext) (types.GateVerdict, error)
	ExecuteBlockFn    func(context.Context, types.FinalizedBlock) (types.BlockOutcome, error)
	CommitFn          func(context.Context) (types.CommitResult, error)
	QueryFn           func(context.Context, types.StateQuery) (types.StateQueryResult, error)
	BuildProposalFn   func(context.Context, types.ProposalContext) (types.BuiltProposal, error)
	VerifyProposalFn  func(context.Context, types.ReceivedProposal) (types.ProposalVerdict, error)
	ExtendVoteFn      func(context.Context, types.VoteContext) ([]byte, error)
	VerifyExtensionFn func(context.Context, types.ReceivedExtension) (types.ExtensionVerdict, error)
	AvailableSnapshotsFn func(context.Context) ([]types.SnapshotDescriptor, error)
	ExportSnapshotFn  func(context.Context, uint64, uint32) (<-chan types.SnapshotChunk, *types.SnapshotDescriptor, error)
	ImportSnapshotFn  func(context.Context, types.SnapshotDescriptor, <-chan types.SnapshotChunk) (types.ImportResult, error)
	SimulateFn        func(context.Context, types.Tx) (types.TxOutcome, error)

	// Call counters (atomic for concurrent access).
	HandshakeCalls    atomic.Int64
	CheckTxCalls      atomic.Int64
	ExecuteBlockCalls atomic.Int64
	CommitCalls       atomic.Int64
	QueryCalls        atomic.Int64
}

func (m *MockApp) Handshake(ctx context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error) {
	m.HandshakeCalls.Add(1)
	if m.HandshakeFn != nil {
		return m.HandshakeFn(ctx, req)
	}
	return types.HandshakeResponse{
		Capabilities: m.DeclaredCapabilities,
	}, nil
}

func (m *MockApp) CheckTx(ctx context.Context, tx types.Tx, mctx types.MempoolContext) (types.GateVerdict, error) {
	m.CheckTxCalls.Add(1)
	if m.CheckTxFn != nil {
		return m.CheckTxFn(ctx, tx, mctx)
	}
	return types.GateVerdict{Code: 0}, nil
}

func (m *MockApp) ExecuteBlock(ctx context.Context, block types.FinalizedBlock) (types.BlockOutcome, error) {
	m.ExecuteBlockCalls.Add(1)
	if m.ExecuteBlockFn != nil {
		return m.ExecuteBlockFn(ctx, block)
	}
	outcomes := make([]types.TxOutcome, len(block.Txs))
	for i := range block.Txs {
		outcomes[i] = types.TxOutcome{Index: uint32(i), Code: 0}
	}
	return types.BlockOutcome{
		TxOutcomes: outcomes,
		AppHash:    types.AppHash{0x01},
	}, nil
}

func (m *MockApp) Commit(ctx context.Context) (types.CommitResult, error) {
	m.CommitCalls.Add(1)
	if m.CommitFn != nil {
		return m.CommitFn(ctx)
	}
	return types.CommitResult{}, nil
}

func (m *MockApp) Query(ctx context.Context, req types.StateQuery) (types.StateQueryResult, error) {
	m.QueryCalls.Add(1)
	if m.QueryFn != nil {
		return m.QueryFn(ctx, req)
	}
	return types.StateQueryResult{}, nil
}

func (m *MockApp) BuildProposal(ctx context.Context, pctx types.ProposalContext) (types.BuiltProposal, error) {
	if m.BuildProposalFn != nil {
		return m.BuildProposalFn(ctx, pctx)
	}
	return types.BuiltProposal{Txs: pctx.MempoolTxs}, nil
}

func (m *MockApp) VerifyProposal(ctx context.Context, prop types.ReceivedProposal) (types.ProposalVerdict, error) {
	if m.VerifyProposalFn != nil {
		return m.VerifyProposalFn(ctx, prop)
	}
	return types.ProposalVerdict{Accept: true}, nil
}

func (m *MockApp) ExtendVote(ctx context.Context, vctx types.VoteContext) ([]byte, error) {
	if m.ExtendVoteFn != nil {
		return m.ExtendVoteFn(ctx, vctx)
	}
	return nil, nil
}

func (m *MockApp) VerifyExtension(ctx context.Context, ext types.ReceivedExtension) (types.ExtensionVerdict, error) {
	if m.VerifyExtensionFn != nil {
		return m.VerifyExtensionFn(ctx, ext)
	}
	return types.ExtensionAccept, nil
}

func (m *MockApp) AvailableSnapshots(ctx context.Context) ([]types.SnapshotDescriptor, error) {
	if m.AvailableSnapshotsFn != nil {
		return m.AvailableSnapshotsFn(ctx)
	}
	return nil, nil
}

func (m *MockApp) ExportSnapshot(ctx context.Context, height uint64, format uint32) (<-chan types.SnapshotChunk, *types.SnapshotDescriptor, error) {
	if m.ExportSnapshotFn != nil {
		return m.ExportSnapshotFn(ctx, height, format)
	}
	ch := make(chan types.SnapshotChunk)
	close(ch)
	return ch, &types.SnapshotDescriptor{Height: height, Format: format}, nil
}

func (m *MockApp) ImportSnapshot(ctx context.Context, desc types.SnapshotDescriptor, chunks <-chan types.SnapshotChunk) (types.ImportResult, error) {
	if m.ImportSnapshotFn != nil {
		return m.ImportSnapshotFn(ctx, desc, chunks)
	}
	// Drain the channel.
	for range chunks {
	}
	ah := types.AppHash{0x01}
	return types.ImportResult{Status: types.ImportOK, AppHash: &ah}, nil
}

func (m *MockApp) Simulate(ctx context.Context, tx types.Tx) (types.TxOutcome, error) {
	if m.SimulateFn != nil {
		return m.SimulateFn(ctx, tx)
	}
	return types.TxOutcome{Code: 0}, nil
}
