// Package bapi defines the Block Application Programming Interface —
// a ground-up redesign of the consensus ↔ application boundary.
//
// The core [Lifecycle] interface is required. All other interfaces
// are optional capabilities discovered via Go type assertion at
// handshake time.
package bapi

import (
	"context"

	"github.com/blockberries/bapi/types"
)

// Lifecycle is the core interface every BAPI application must implement.
// It covers the complete happy-path from boot to steady-state block
// production.
//
// The engine guarantees the following call order:
//  1. Handshake is called exactly once, before anything else.
//  2. ExecuteBlock(h) is called exactly once per committed height h.
//  3. Commit is called exactly once after each ExecuteBlock.
//  4. CheckTx, Query may be called concurrently at any time after Handshake.
type Lifecycle interface {
	// Handshake is called once on every startup (cold start or restart).
	//
	// The engine communicates the last block it committed. If LastCommitted
	// is nil, this is a fresh genesis and Genesis will be populated.
	//
	// The application returns its own view of its state so the engine can
	// detect and recover from any divergence.
	//
	// Replaces: ABCI Info + InitChain
	Handshake(ctx context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error)

	// CheckTx gate-checks a transaction before it enters the mempool.
	//
	// The context parameter distinguishes first-seen transactions from
	// periodic re-validations after state changes.
	//
	// This method MUST be safe for concurrent use.
	//
	// Replaces: ABCI CheckTx (with type enum)
	CheckTx(ctx context.Context, tx types.Tx, mctx types.MempoolContext) (types.GateVerdict, error)

	// ExecuteBlock deterministically executes a finalized block.
	//
	// Called after consensus has been reached. The application MUST execute
	// every transaction in order and return a comprehensive BlockOutcome.
	//
	// This method MUST NOT persist state to disk — that happens in Commit.
	// The AppHash in the returned BlockOutcome must be deterministic: all
	// correct nodes executing the same block must produce the same AppHash.
	//
	// Replaces: ABCI FinalizeBlock
	ExecuteBlock(ctx context.Context, block types.FinalizedBlock) (types.BlockOutcome, error)

	// Commit persists all state changes from the last ExecuteBlock to
	// durable storage.
	//
	// Called exactly once after each ExecuteBlock. Must be crash-safe:
	// either all changes land, or none do (atomic persistence).
	//
	// Returns the retain height — the minimum height the engine should
	// keep for historical queries. The engine MAY prune blocks below this.
	//
	// Replaces: ABCI Commit
	Commit(ctx context.Context) (types.CommitResult, error)

	// Query reads application state.
	//
	// Supports both current and historical queries (if the app retains
	// history) and optional Merkle proofs for light client verification.
	//
	// This method MUST be safe for concurrent use, including concurrent
	// with ExecuteBlock (reads should see the last committed state).
	//
	// Replaces: ABCI Query
	Query(ctx context.Context, req types.StateQuery) (types.StateQueryResult, error)
}

// ProposalControl allows the application to influence which transactions
// appear in a block and in what order. If the application does not
// implement this interface, the engine fills blocks from its mempool
// in priority order.
//
// Declared via: types.CapProposalControl in HandshakeResponse.Capabilities
type ProposalControl interface {
	// BuildProposal is called when this validator is the proposer.
	//
	// The engine provides mempool contents and size limits. The application
	// returns an ordered list of transactions. The application may reorder,
	// drop, or inject transactions (e.g., oracle price updates, forced
	// liquidations).
	//
	// Replaces: ABCI PrepareProposal
	BuildProposal(ctx context.Context, pctx types.ProposalContext) (types.BuiltProposal, error)

	// VerifyProposal is called on every validator for a received proposal.
	//
	// This runs the application's structural validation. It MUST NOT
	// execute transactions (that happens in ExecuteBlock). Returning a
	// rejection causes the validator to nil-prevote.
	//
	// This method MUST be deterministic: all honest validators must reach
	// the same verdict for the same proposal.
	//
	// Replaces: ABCI ProcessProposal
	VerifyProposal(ctx context.Context, proposal types.ReceivedProposal) (types.ProposalVerdict, error)
}

// VoteExtender enables validators to attach arbitrary signed data to
// their precommit votes. This powers use cases like oracle price feeds,
// threshold signatures, and availability attestations.
//
// Declared via: types.CapVoteExtensions in HandshakeResponse.Capabilities
type VoteExtender interface {
	// ExtendVote produces an extension to attach to this validator's
	// precommit vote at the given height/round.
	//
	// The extension is opaque bytes — the application defines the schema.
	// Extensions are propagated to all validators and included in the
	// next block's FinalizedBlock.VoteExtensions.
	//
	// Replaces: ABCI ExtendVote
	ExtendVote(ctx context.Context, vctx types.VoteContext) ([]byte, error)

	// VerifyExtension validates another validator's vote extension.
	//
	// MUST be deterministic across all honest validators. Returning
	// Reject causes the vote to be ignored during consensus.
	//
	// Replaces: ABCI VerifyVoteExtension
	VerifyExtension(ctx context.Context, ext types.ReceivedExtension) (types.ExtensionVerdict, error)
}

// StateSync enables snapshot-based state synchronization for fast node
// bootstrapping. Replaces the four ABCI snapshot methods with a
// streamlined export/import model.
//
// Declared via: types.CapStateSync in HandshakeResponse.Capabilities
type StateSync interface {
	// AvailableSnapshots lists snapshots the application can export.
	//
	// Replaces: ABCI ListSnapshots
	AvailableSnapshots(ctx context.Context) ([]types.SnapshotDescriptor, error)

	// ExportSnapshot exports a snapshot as a pull-based stream of chunks.
	//
	// The returned channel yields chunks in order. The channel is closed
	// after the last chunk. The caller controls backpressure by the rate
	// at which it reads from the channel.
	//
	// Replaces: ABCI ListSnapshots + LoadSnapshotChunk
	ExportSnapshot(ctx context.Context, height uint64, format uint32) (<-chan types.SnapshotChunk, *types.SnapshotDescriptor, error)

	// ImportSnapshot imports a snapshot from a push-based stream of chunks.
	//
	// The application receives the descriptor and a channel of chunks.
	// After consuming all chunks, it must rebuild state and return the
	// resulting AppHash so the engine can verify it against the chain.
	//
	// Replaces: ABCI OfferSnapshot + ApplySnapshotChunk
	ImportSnapshot(ctx context.Context, descriptor types.SnapshotDescriptor, chunks <-chan types.SnapshotChunk) (types.ImportResult, error)
}

// Simulator provides a clean, dedicated path for dry-run execution.
// In ABCI 2.0, this was overloaded onto CheckTx or required out-of-band
// RPC.
//
// Declared via: types.CapSimulation in HandshakeResponse.Capabilities
type Simulator interface {
	// Simulate dry-runs a transaction against current committed state
	// without persisting any changes. Returns the execution result
	// including events and application-defined result data.
	//
	// This method MUST be safe for concurrent use.
	Simulate(ctx context.Context, tx types.Tx) (types.TxOutcome, error)
}

// Application is a convenience interface that embeds all BAPI
// interfaces. Applications that support all capabilities may implement
// this directly. Most applications should implement only Lifecycle plus
// the optional interfaces they need.
type Application interface {
	Lifecycle
	ProposalControl
	VoteExtender
	StateSync
	Simulator
}

// Connection represents a transport-agnostic connection to a BAPI
// application. Both gRPC clients and in-process adapters implement this.
type Connection interface {
	Lifecycle

	// Capabilities returns the capabilities discovered at handshake.
	// Must only be called after Handshake completes.
	Capabilities() types.Capabilities

	// AsProposalControl returns the ProposalControl interface if
	// available, or nil if the app does not support it.
	AsProposalControl() ProposalControl

	// AsVoteExtender returns the VoteExtender interface if available.
	AsVoteExtender() VoteExtender

	// AsStateSync returns the StateSync interface if available.
	AsStateSync() StateSync

	// AsSimulator returns the Simulator interface if available.
	AsSimulator() Simulator

	// Close terminates the connection.
	Close() error
}
