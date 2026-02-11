package server

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/blockberries/bapi"
	"github.com/blockberries/bapi/types"
)

// Server wraps a BAPI application with lifecycle enforcement
// and capability routing. The consensus engine interacts with
// the application exclusively through this server.
type Server struct {
	app   bapi.Lifecycle
	guard *LifecycleGuard
	caps  types.Capabilities

	// Optional interfaces (nil if not supported).
	proposalCtl bapi.ProposalControl
	voteExt     bapi.VoteExtender
	stateSync   bapi.StateSync
	simulator   bapi.Simulator

	// Last block outcome (held between ExecuteBlock and Commit).
	mu             sync.Mutex
	lastOutcome    *types.BlockOutcome
	lastExecHeight uint64
}

// New creates a new Server wrapping the given application.
func New(app bapi.Lifecycle) *Server {
	s := &Server{
		app:   app,
		guard: NewLifecycleGuard(),
	}
	// Pre-discover optional interfaces (validated after handshake).
	s.proposalCtl, _ = app.(bapi.ProposalControl)
	s.voteExt, _ = app.(bapi.VoteExtender)
	s.stateSync, _ = app.(bapi.StateSync)
	s.simulator, _ = app.(bapi.Simulator)
	return s
}

// Handshake performs the startup handshake, validates capability
// declarations, and transitions the state machine to Ready.
func (s *Server) Handshake(ctx context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error) {
	s.guard.AcquireHandshake()

	resp, err := s.app.Handshake(ctx, req)
	if err != nil {
		s.guard.FailHandshake()
		return resp, err
	}

	if err := discoverCapabilities(s.app, resp.Capabilities); err != nil {
		s.guard.FailHandshake()
		return resp, err
	}

	s.caps = resp.Capabilities
	s.guard.CompleteHandshake()
	return resp, nil
}

// CheckTx gate-checks a transaction for mempool admission.
// Safe for concurrent use.
func (s *Server) CheckTx(ctx context.Context, tx types.Tx, mctx types.MempoolContext) (types.GateVerdict, error) {
	s.guard.CheckConcurrent()
	return s.app.CheckTx(ctx, tx, mctx)
}

// ExecuteBlock deterministically executes a finalized block.
func (s *Server) ExecuteBlock(ctx context.Context, block types.FinalizedBlock) (types.BlockOutcome, error) {
	s.guard.AcquireExecute()

	outcome, err := s.app.ExecuteBlock(ctx, block)
	if err != nil {
		s.guard.FailExecute()
		return outcome, err
	}

	s.mu.Lock()
	s.lastOutcome = &outcome
	s.lastExecHeight = block.Height
	s.mu.Unlock()

	s.guard.CompleteExecute()
	return outcome, nil
}

// Commit persists state changes from the last ExecuteBlock.
func (s *Server) Commit(ctx context.Context) (types.CommitResult, error) {
	s.guard.AcquireCommit()

	result, err := s.app.Commit(ctx)

	s.mu.Lock()
	s.lastOutcome = nil
	s.mu.Unlock()

	s.guard.CompleteCommit()
	return result, err
}

// Query reads application state. Safe for concurrent use.
func (s *Server) Query(ctx context.Context, req types.StateQuery) (types.StateQueryResult, error) {
	s.guard.CheckConcurrent()
	return s.app.Query(ctx, req)
}

// Capabilities returns the application's declared capabilities.
// Only valid after Handshake completes.
func (s *Server) Capabilities() types.Capabilities {
	return s.caps
}

// --- Capability-gated optional methods ---

// BuildProposal delegates to ProposalControl if supported.
func (s *Server) BuildProposal(ctx context.Context, pctx types.ProposalContext) (types.BuiltProposal, error) {
	if s.proposalCtl == nil {
		return types.BuiltProposal{}, fmt.Errorf("github.com/blockberries/bapi: ProposalControl not supported")
	}
	return s.proposalCtl.BuildProposal(ctx, pctx)
}

// VerifyProposal delegates to ProposalControl if supported.
// Returns Accept by default if not supported.
func (s *Server) VerifyProposal(ctx context.Context, prop types.ReceivedProposal) (types.ProposalVerdict, error) {
	if s.proposalCtl == nil {
		return types.ProposalVerdict{Accept: true}, nil
	}
	return s.proposalCtl.VerifyProposal(ctx, prop)
}

// ExtendVote delegates to VoteExtender if supported.
func (s *Server) ExtendVote(ctx context.Context, vctx types.VoteContext) ([]byte, error) {
	if s.voteExt == nil {
		return nil, fmt.Errorf("github.com/blockberries/bapi: VoteExtender not supported")
	}
	return s.voteExt.ExtendVote(ctx, vctx)
}

// VerifyExtension delegates to VoteExtender if supported.
// Returns Accept by default if not supported.
func (s *Server) VerifyExtension(ctx context.Context, ext types.ReceivedExtension) (types.ExtensionVerdict, error) {
	if s.voteExt == nil {
		return types.ExtensionAccept, nil
	}
	return s.voteExt.VerifyExtension(ctx, ext)
}

// AvailableSnapshots delegates to StateSync if supported.
func (s *Server) AvailableSnapshots(ctx context.Context) ([]types.SnapshotDescriptor, error) {
	if s.stateSync == nil {
		return nil, fmt.Errorf("github.com/blockberries/bapi: StateSync not supported")
	}
	return s.stateSync.AvailableSnapshots(ctx)
}

// ExportSnapshot delegates to StateSync if supported.
func (s *Server) ExportSnapshot(ctx context.Context, height uint64, format uint32) (<-chan types.SnapshotChunk, *types.SnapshotDescriptor, error) {
	if s.stateSync == nil {
		return nil, nil, fmt.Errorf("github.com/blockberries/bapi: StateSync not supported")
	}
	return s.stateSync.ExportSnapshot(ctx, height, format)
}

// ImportSnapshot delegates to StateSync if supported.
func (s *Server) ImportSnapshot(ctx context.Context, desc types.SnapshotDescriptor, chunks <-chan types.SnapshotChunk) (types.ImportResult, error) {
	if s.stateSync == nil {
		return types.ImportResult{}, fmt.Errorf("github.com/blockberries/bapi: StateSync not supported")
	}
	return s.stateSync.ImportSnapshot(ctx, desc, chunks)
}

// Simulate delegates to Simulator if supported.
// Safe for concurrent use.
func (s *Server) Simulate(ctx context.Context, tx types.Tx) (types.TxOutcome, error) {
	if s.simulator == nil {
		return types.TxOutcome{}, fmt.Errorf("github.com/blockberries/bapi: Simulator not supported")
	}
	s.guard.CheckConcurrent()
	return s.simulator.Simulate(ctx, tx)
}

// AsProposalControl returns the ProposalControl interface or nil.
func (s *Server) AsProposalControl() bapi.ProposalControl {
	if s.caps.Has(types.CapProposalControl) {
		return s.proposalCtl
	}
	return nil
}

// AsVoteExtender returns the VoteExtender interface or nil.
func (s *Server) AsVoteExtender() bapi.VoteExtender {
	if s.caps.Has(types.CapVoteExtensions) {
		return s.voteExt
	}
	return nil
}

// AsStateSync returns the StateSync interface or nil.
func (s *Server) AsStateSync() bapi.StateSync {
	if s.caps.Has(types.CapStateSync) {
		return s.stateSync
	}
	return nil
}

// AsSimulator returns the Simulator interface or nil.
func (s *Server) AsSimulator() bapi.Simulator {
	if s.caps.Has(types.CapSimulation) {
		return s.simulator
	}
	return nil
}

// LastOutcome returns the most recent BlockOutcome (between
// ExecuteBlock and Commit). Returns nil if no outcome is pending.
func (s *Server) LastOutcome() *types.BlockOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastOutcome
}

// Close is a no-op for the server wrapper.
func (s *Server) Close() error { return nil }

// discoverCapabilities checks which optional interfaces the app
// implements and verifies consistency with declared capabilities.
func discoverCapabilities(app bapi.Lifecycle, declared types.Capabilities) error {
	_, hasProposal := app.(bapi.ProposalControl)
	_, hasVoteExt := app.(bapi.VoteExtender)
	_, hasStateSync := app.(bapi.StateSync)
	_, hasSimulator := app.(bapi.Simulator)

	if declared.Has(types.CapProposalControl) && !hasProposal {
		return fmt.Errorf("github.com/blockberries/bapi: app declared CapProposalControl but does not implement ProposalControl")
	}
	if declared.Has(types.CapVoteExtensions) && !hasVoteExt {
		return fmt.Errorf("github.com/blockberries/bapi: app declared CapVoteExtensions but does not implement VoteExtender")
	}
	if declared.Has(types.CapStateSync) && !hasStateSync {
		return fmt.Errorf("github.com/blockberries/bapi: app declared CapStateSync but does not implement StateSync")
	}
	if declared.Has(types.CapSimulation) && !hasSimulator {
		return fmt.Errorf("github.com/blockberries/bapi: app declared CapSimulation but does not implement Simulator")
	}

	// Warn (but don't error) if the app implements an interface but didn't declare it.
	if !declared.Has(types.CapProposalControl) && hasProposal {
		log.Println("github.com/blockberries/bapi: WARNING: app implements ProposalControl but did not declare it; capability will not be used")
	}
	if !declared.Has(types.CapVoteExtensions) && hasVoteExt {
		log.Println("github.com/blockberries/bapi: WARNING: app implements VoteExtender but did not declare it; capability will not be used")
	}
	if !declared.Has(types.CapStateSync) && hasStateSync {
		log.Println("github.com/blockberries/bapi: WARNING: app implements StateSync but did not declare it; capability will not be used")
	}
	if !declared.Has(types.CapSimulation) && hasSimulator {
		log.Println("github.com/blockberries/bapi: WARNING: app implements Simulator but did not declare it; capability will not be used")
	}

	return nil
}
