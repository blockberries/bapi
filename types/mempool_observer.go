package types

// BatchCertifiedEvent is the payload of MempoolObserver.OnBatchCertified.
// Fired when a mempool batch reaches cert-quorum — that is, the header
// referencing this batch was vote-certified into a DAG round cert (the
// durable, committed-to-DAG event; not the earlier ack-quorum gate).
//
// Tokenomics modules use this to count batches-certified-per-validator
// per epoch (see the spec §7 Participation Tracker). The participation
// tracker is the only consumer in v1; other consumers may attach later.
//
// The payload is deliberately narrow — see PLAN §7 decision D15. The
// full batch contents are NOT carried; consumers needing them must
// look up by BatchHash through the consensus layer's batch store.
type BatchCertifiedEvent struct {
	// Validator is the address of the validator that authored this
	// batch (the batch's primary, in looseberry vocab).
	Validator ValidatorAddress `cramberry:"1"`
	// BatchHash uniquely identifies the batch. Stable; usable as a
	// store key.
	BatchHash Hash `cramberry:"2"`
	// Round is the DAG round at which this batch's cert was added.
	Round uint64 `cramberry:"3"`
	// TxCount is the number of transactions in the batch.
	TxCount uint32 `cramberry:"4"`
	// ByteCount is the total byte size of the batch's transaction
	// payloads. Useful for byte-fee-revenue accounting.
	ByteCount uint64 `cramberry:"5"`
}

// BlockConstructedEvent is the payload of MempoolObserver.OnBlockConstructed.
// Fired when leaderberry assembles a block (after proposal building,
// before broadcast). The leader is the validator that proposed this
// block at this height.
//
// Tokenomics consumers count leader_blocks_v[leader] only when
// `len(IncludedBatchHashes) >= 1`. An empty-block proposal (no
// certified batches at construction time, or proposer chose to skip)
// earns no leader credit per PLAN §7 decision D11. The participation
// tracker enforces this.
//
// The leader's batches-certified count is tracked separately via
// OnBatchCertified; this event is purely about who proposed which
// block.
type BlockConstructedEvent struct {
	// Leader is the address of the validator that proposed this
	// block. Same identity as the eventual FinalizedBlock.Proposer.
	Leader ValidatorAddress `cramberry:"1"`
	// Height of the block being constructed.
	Height uint64 `cramberry:"2"`
	// IncludedBatchHashes is the set of batch identifiers included
	// in this block's data section. Empty means a block with no
	// certified batches; per D11 no leader credit accrues.
	IncludedBatchHashes []Hash `cramberry:"3"`
}
