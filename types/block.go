package types

// TxOutcome is the result of executing a single transaction.
type TxOutcome struct {
	// Position of this tx in the block (0-indexed).
	Index uint32 `cramberry:"1"`
	// Application-defined result code. 0 = success.
	Code uint32 `cramberry:"2"`
	// Human-readable result info (non-deterministic, for debugging).
	Info string `cramberry:"3"`
	// Application-defined data returned from execution (deterministic).
	Data []byte `cramberry:"4"`
	// Events emitted by this transaction.
	Events []Event `cramberry:"5"`
}

// OK returns true if the transaction executed successfully.
func (t TxOutcome) OK() bool { return t.Code == 0 }

// BlockOutcome is the comprehensive output of executing a finalized block.
// This is the keystone type of BAPI — all execution side-effects live here.
type BlockOutcome struct {
	// Per-transaction results, in block order.
	TxOutcomes []TxOutcome `cramberry:"1"`
	// Block-level events (rewards, slashing, epoch transitions, etc.).
	BlockEvents []Event `cramberry:"2"`
	// New app state root after this block.
	AppHash AppHash `cramberry:"3"`
	// Changes to the validator set. Empty slice = no change.
	ValidatorUpdates []ValidatorUpdate `cramberry:"4"`
	// Changes to consensus params. Nil = no change.
	ParamsUpdate *ConsensusParams `cramberry:"5"`
}

// FinalizedBlock is a decided block delivered to the application
// for execution.
type FinalizedBlock struct {
	Height        uint64           `cramberry:"1"`
	Time          Timestamp        `cramberry:"2"`
	Proposer      ValidatorAddress `cramberry:"3"`
	Txs           []Tx             `cramberry:"4"`
	LastBlockHash Hash             `cramberry:"5"`
	Evidence      []Evidence       `cramberry:"6"`
	// Committed vote extensions from the previous height.
	// Only populated if the app declared CapVoteExtensions.
	VoteExtensions []CommittedVoteExtension `cramberry:"7"`
}

// CommitResult is returned after the application persists
// state to disk.
type CommitResult struct {
	// Minimum height the app still needs for queries / proofs.
	// The engine may prune blocks below this. 0 = no pruning preference.
	RetainHeight uint64 `cramberry:"1"`
}
