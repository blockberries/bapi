package types

// EvidenceType identifies the kind of Byzantine behavior.
type EvidenceType uint8

const (
	EvidenceTypeDuplicateVote EvidenceType = 1
	EvidenceTypeLightClient   EvidenceType = 2
)

// Evidence represents proof of Byzantine behavior.
type Evidence struct {
	Type             EvidenceType     `cramberry:"1"`
	Validator        ValidatorAddress `cramberry:"2"`
	Height           uint64           `cramberry:"3"`
	Time             Timestamp        `cramberry:"4"`
	TotalVotingPower uint64           `cramberry:"5"`
}
