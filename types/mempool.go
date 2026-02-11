package types

// MempoolContext tells the application whether a transaction
// is being seen for the first time or is being re-validated.
type MempoolContext uint8

const (
	// MempoolFirstSeen indicates the transaction was just received.
	MempoolFirstSeen MempoolContext = 1
	// MempoolRevalidation indicates the transaction is being
	// re-checked after state changed (e.g., a new block was committed).
	MempoolRevalidation MempoolContext = 2
)

// GateVerdict is the application's decision on whether a
// transaction should be admitted to the mempool.
type GateVerdict struct {
	// 0 = accepted into mempool. Non-zero = rejected.
	Code uint32 `cramberry:"1"`
	// Rejection reason (debugging only, non-deterministic).
	Info string `cramberry:"2"`
	// Priority for ordering within the mempool. Higher = first.
	Priority int64 `cramberry:"3"`
	// Sender identifier for same-sender sequencing / replacement.
	Sender string `cramberry:"4"`
}

// Accepted returns true if the transaction was admitted.
func (v GateVerdict) Accepted() bool { return v.Code == 0 }
