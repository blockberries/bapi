package bapi

import (
	"errors"
	"fmt"
)

// HaltError signals that the application detected an irrecoverable
// inconsistency and requests an immediate chain halt.
//
// When the engine receives a HaltError from ExecuteBlock, it must
// stop consensus, log the error, and not proceed to Commit.
type HaltError struct {
	Reason string
	Height uint64
}

func (e *HaltError) Error() string {
	return fmt.Sprintf("HALT at height %d: %s", e.Height, e.Reason)
}

// NewHaltError creates a new HaltError.
func NewHaltError(height uint64, reason string) *HaltError {
	return &HaltError{Height: height, Reason: reason}
}

// IsHalt checks whether an error is a HaltError and returns it.
func IsHalt(err error) (*HaltError, bool) {
	var h *HaltError
	if errors.As(err, &h) {
		return h, true
	}
	return nil, false
}
