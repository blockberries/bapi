package types

import "strings"

// Capabilities is a bitfield declaring which optional interfaces
// the application supports.
type Capabilities uint8

const (
	CapProposalControl Capabilities = 1 << iota // 0b0001
	CapVoteExtensions                            // 0b0010
	CapStateSync                                 // 0b0100
	CapSimulation                                // 0b1000
)

// Has returns true if all bits in cap are set.
func (c Capabilities) Has(cap Capabilities) bool {
	return c&cap == cap
}

// String returns a human-readable representation.
func (c Capabilities) String() string {
	var caps []string
	if c.Has(CapProposalControl) {
		caps = append(caps, "ProposalControl")
	}
	if c.Has(CapVoteExtensions) {
		caps = append(caps, "VoteExtensions")
	}
	if c.Has(CapStateSync) {
		caps = append(caps, "StateSync")
	}
	if c.Has(CapSimulation) {
		caps = append(caps, "Simulation")
	}
	if len(caps) == 0 {
		return "none"
	}
	return strings.Join(caps, "|")
}
