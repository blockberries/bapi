package types

// ProposalContext is provided to the application when it is
// the block proposer.
type ProposalContext struct {
	Height   uint64           `cramberry:"1"`
	Time     Timestamp        `cramberry:"2"`
	Proposer ValidatorAddress `cramberry:"3"`
	// Transactions from the mempool, pre-sorted by priority.
	MempoolTxs []Tx `cramberry:"4"`
	// Maximum total bytes for the block's tx payload.
	MaxTxBytes uint64 `cramberry:"5"`
	// Vote extensions from the previous round (if applicable).
	VoteExtensions []CommittedVoteExtension `cramberry:"6"`
}

// BuiltProposal is the application's assembled block contents.
type BuiltProposal struct {
	Txs []Tx `cramberry:"1"`
}

// ReceivedProposal is a proposal received from another
// validator for verification.
type ReceivedProposal struct {
	Height         uint64           `cramberry:"1"`
	Time           Timestamp        `cramberry:"2"`
	Proposer       ValidatorAddress `cramberry:"3"`
	Txs            []Tx             `cramberry:"4"`
	VoteExtensions []CommittedVoteExtension `cramberry:"5"`
}

// ProposalVerdict is the application's decision on a
// received proposal.
type ProposalVerdict struct {
	// Accept is true if the proposal is structurally valid.
	Accept bool `cramberry:"1"`
	// Reason for rejection (only set when Accept is false).
	RejectReason string `cramberry:"2"`
}
