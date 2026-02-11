package types

// StateQuery is a request to read application state.
type StateQuery struct {
	Path QueryPath `cramberry:"1"`
	Data []byte    `cramberry:"2"`
	// Height to query at. Nil = latest committed state.
	Height *uint64 `cramberry:"3"`
	// If true, include a Merkle proof in the result.
	Prove bool `cramberry:"4"`
}

// StateQueryResult is the application's response to a state query.
type StateQueryResult struct {
	Code   uint32       `cramberry:"1"`
	Key    []byte       `cramberry:"2"`
	Value  []byte       `cramberry:"3"`
	Height uint64       `cramberry:"4"`
	Proof  *MerkleProof `cramberry:"5"`
	Info   string       `cramberry:"6"`
}

// MerkleProof is an inclusion/exclusion proof against the
// app state root.
type MerkleProof struct {
	Ops []ProofOp `cramberry:"1"`
}

// ProofOp is a single operation in a Merkle proof.
type ProofOp struct {
	Type string `cramberry:"1"`
	Key  []byte `cramberry:"2"`
	Data []byte `cramberry:"3"`
}
