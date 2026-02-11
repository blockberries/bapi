// Package types defines all core data types for the BAPI
// (Block Application Programming Interface).
//
// These are plain Go structs with cramberry struct tags for
// deterministic binary serialization. Transport concerns
// (gRPC codec registration) are handled in the transport packages.
package types

// Hash is a 32-byte cryptographic hash.
type Hash [32]byte

// AppHash is a deterministic fingerprint of the application
// state after execution.
type AppHash [32]byte

// Tx is an opaque application transaction.
// The consensus engine never inspects its contents.
type Tx []byte

// QueryPath is a structured key for state queries
// (e.g., "/store/bank/balance/cosmos1...").
type QueryPath string

// BlockID uniquely identifies a point in the chain.
type BlockID struct {
	Height uint64 `cramberry:"1"`
	Hash   Hash   `cramberry:"2"`
}
