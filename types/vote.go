package types

// VoteContext provides the context for producing a vote extension.
type VoteContext struct {
	Height    uint64 `cramberry:"1"`
	Round     uint32 `cramberry:"2"`
	BlockHash Hash   `cramberry:"3"`
}

// ReceivedExtension is another validator's vote extension to verify.
type ReceivedExtension struct {
	Height    uint64           `cramberry:"1"`
	Round     uint32           `cramberry:"2"`
	Validator ValidatorAddress `cramberry:"3"`
	Extension []byte           `cramberry:"4"`
}

// ExtensionVerdict is the application's decision on a received
// vote extension.
type ExtensionVerdict uint8

const (
	ExtensionAccept ExtensionVerdict = 1
	ExtensionReject ExtensionVerdict = 2
)

// CommittedVoteExtension is an extension that survived
// verification and was committed.
type CommittedVoteExtension struct {
	Validator ValidatorAddress `cramberry:"1"`
	Extension []byte           `cramberry:"2"`
}
