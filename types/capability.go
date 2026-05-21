package types

import "strings"

// Capabilities is a bitfield declaring which optional interfaces
// the application supports.
type Capabilities uint8

const (
	CapProposalControl Capabilities = 1 << iota // 0b00001
	CapVoteExtensions                            // 0b00010
	CapStateSync                                 // 0b00100
	CapSimulation                                // 0b01000
	// CapMempoolObserver gates the MempoolObserver capability —
	// callbacks the consensus engine fires when a mempool batch reaches
	// cert-quorum and when a block is constructed. Used by tokenomics
	// modules to drive a participation tracker (per-epoch leader-block
	// + batches-certified counts) and to route per-block fee
	// settlement. See the bapi.MempoolObserver interface.
	CapMempoolObserver // 0b10000
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
	if c.Has(CapMempoolObserver) {
		caps = append(caps, "MempoolObserver")
	}
	if len(caps) == 0 {
		return "none"
	}
	return strings.Join(caps, "|")
}
