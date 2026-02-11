package types

// ConsensusParams contains consensus-critical parameters
// the application can request to change.
type ConsensusParams struct {
	MaxBlockBytes  uint64   `cramberry:"1"`
	MaxEvidenceAge Duration `cramberry:"2"`
	MaxTxBytes     uint64   `cramberry:"3"`
}
