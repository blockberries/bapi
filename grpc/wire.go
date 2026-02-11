package bapigrpc

import "bapi/types"

// Transport-specific wrapper types for RPC methods whose interface
// signatures don't map to a single request/response struct.
// These are used only for gRPC serialization boundaries.

// CheckTxRequest wraps the parameters for Lifecycle.CheckTx.
type CheckTxRequest struct {
	Tx      types.Tx            `cramberry:"1"`
	Context types.MempoolContext `cramberry:"2"`
}

// CommitRequest is the (empty) request for Lifecycle.Commit.
type CommitRequest struct{}

// ExtendVoteResponse wraps the return value of VoteExtender.ExtendVote.
type ExtendVoteResponse struct {
	Extension []byte `cramberry:"1"`
}

// ExtensionVerdictResponse wraps the return value of VoteExtender.VerifyExtension.
type ExtensionVerdictResponse struct {
	Verdict types.ExtensionVerdict `cramberry:"1"`
}

// AvailableSnapshotsRequest is the (empty) request for StateSync.AvailableSnapshots.
type AvailableSnapshotsRequest struct{}

// AvailableSnapshotsResponse wraps the return value of StateSync.AvailableSnapshots.
type AvailableSnapshotsResponse struct {
	Snapshots []types.SnapshotDescriptor `cramberry:"1"`
}

// ExportSnapshotRequest wraps parameters for StateSync.ExportSnapshot.
type ExportSnapshotRequest struct {
	Height uint64 `cramberry:"1"`
	Format uint32 `cramberry:"2"`
}

// ImportSnapshotMessage is a tagged union carrying either a descriptor
// (first message) or a chunk (subsequent messages) for the
// ImportSnapshot client-streaming RPC.
type ImportSnapshotMessage struct {
	Descriptor *types.SnapshotDescriptor `cramberry:"1"`
	Chunk      *types.SnapshotChunk      `cramberry:"2"`
}

// SimulateRequest wraps the parameter for Simulator.Simulate.
type SimulateRequest struct {
	Tx types.Tx `cramberry:"1"`
}
