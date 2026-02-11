package types

// ValidatorAddress is the 20-byte address derived from
// a validator's public key.
type ValidatorAddress [20]byte

// KeyType identifies a cryptographic key algorithm.
type KeyType uint8

const (
	KeyTypeEd25519   KeyType = 1
	KeyTypeSecp256k1 KeyType = 2
)

// PublicKey represents a validator's cryptographic identity.
type PublicKey struct {
	Type KeyType `cramberry:"1"`
	Data []byte  `cramberry:"2"`
}

// ValidatorUpdate represents a change to the validator set.
// Power = 0 means removal of the validator.
type ValidatorUpdate struct {
	PubKey PublicKey `cramberry:"1"`
	Power  uint64   `cramberry:"2"`
}
