package bapigrpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/blockberries/bapi/example/counter"
	"github.com/blockberries/bapi/example/dex"
	bapigrpc "github.com/blockberries/bapi/grpc"
	"github.com/blockberries/bapi/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// startServer starts a gRPC server on a random port and returns
// the listener address and a cleanup function.
func startServer(t *testing.T, gs *bapigrpc.GRPCServer) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	s := grpc.NewServer()
	gs.Register(s)

	go func() {
		if err := s.Serve(lis); err != nil {
			// Ignore errors from graceful stop.
		}
	}()

	return lis.Addr().String(), func() {
		s.GracefulStop()
	}
}

func dial(t *testing.T, addr string) *bapigrpc.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := bapigrpc.Dial(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return client
}

func TestGRPC_Counter_Lifecycle(t *testing.T) {
	gs := bapigrpc.NewGRPCServer(counter.New())
	addr, cleanup := startServer(t, gs)
	defer cleanup()

	client := dial(t, addr)
	defer client.Close()

	ctx := context.Background()

	// Genesis handshake.
	genesis := types.GenesisDoc{
		ChainID:       "test",
		GenesisTime:   types.TimeToTimestamp(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		InitialHeight: 1,
		ConsensusParams: types.ConsensusParams{
			MaxBlockBytes:  1048576,
			MaxTxBytes:     65536,
			MaxEvidenceAge: types.DurationFromGo(24 * time.Hour),
		},
	}
	resp, err := client.Handshake(ctx, types.HandshakeRequest{Genesis: &genesis})
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	if resp.AppHash == nil {
		t.Fatal("expected non-nil AppHash from genesis")
	}
	if resp.Capabilities != 0 {
		t.Fatalf("counter should have no capabilities, got %v", resp.Capabilities)
	}

	// Execute + Commit a block with one increment.
	block := types.FinalizedBlock{
		Height: 1,
		Time:   types.TimeToTimestamp(time.Now()),
		Txs:    []types.Tx{counter.IncrementTx(5)},
	}
	outcome, err := client.ExecuteBlock(ctx, block)
	if err != nil {
		t.Fatalf("ExecuteBlock: %v", err)
	}
	if outcome.AppHash == (types.AppHash{}) {
		t.Fatal("expected non-zero AppHash")
	}
	if len(outcome.TxOutcomes) != 1 || outcome.TxOutcomes[0].Code != 0 {
		t.Fatalf("unexpected tx outcomes: %+v", outcome.TxOutcomes)
	}

	result, err := client.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	_ = result

	// Query.
	qr, err := client.Query(ctx, types.StateQuery{Path: "/count"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if qr.Height != 1 {
		t.Fatalf("expected query height 1, got %d", qr.Height)
	}
}

func TestGRPC_Counter_CheckTx(t *testing.T) {
	gs := bapigrpc.NewGRPCServer(counter.New())
	addr, cleanup := startServer(t, gs)
	defer cleanup()

	client := dial(t, addr)
	defer client.Close()

	ctx := context.Background()

	// Handshake first.
	genesis := types.GenesisDoc{
		ChainID:       "test",
		GenesisTime:   types.TimeToTimestamp(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		InitialHeight: 1,
		ConsensusParams: types.ConsensusParams{
			MaxBlockBytes:  1048576,
			MaxTxBytes:     65536,
			MaxEvidenceAge: types.DurationFromGo(24 * time.Hour),
		},
	}
	if _, err := client.Handshake(ctx, types.HandshakeRequest{Genesis: &genesis}); err != nil {
		t.Fatalf("Handshake: %v", err)
	}

	// Valid tx.
	v, err := client.CheckTx(ctx, counter.IncrementTx(1), types.MempoolFirstSeen)
	if err != nil {
		t.Fatalf("CheckTx: %v", err)
	}
	if !v.Accepted() {
		t.Fatalf("expected accepted, got code %d", v.Code)
	}

	// Invalid tx (wrong size).
	v, err = client.CheckTx(ctx, types.Tx([]byte{0x01}), types.MempoolFirstSeen)
	if err != nil {
		t.Fatalf("CheckTx: %v", err)
	}
	if v.Accepted() {
		t.Fatal("expected rejected, got accepted")
	}
}

func TestGRPC_Dex_AllCapabilities(t *testing.T) {
	gs := bapigrpc.NewGRPCServer(dex.New())
	addr, cleanup := startServer(t, gs)
	defer cleanup()

	client := dial(t, addr)
	defer client.Close()

	ctx := context.Background()

	// Genesis handshake.
	genesis := types.GenesisDoc{
		ChainID:       "dex-test",
		GenesisTime:   types.TimeToTimestamp(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		InitialHeight: 1,
		ConsensusParams: types.ConsensusParams{
			MaxBlockBytes:  1048576,
			MaxTxBytes:     65536,
			MaxEvidenceAge: types.DurationFromGo(24 * time.Hour),
		},
	}
	resp, err := client.Handshake(ctx, types.HandshakeRequest{Genesis: &genesis})
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}

	// Verify all capabilities.
	caps := resp.Capabilities
	if !caps.Has(types.CapProposalControl) {
		t.Error("missing CapProposalControl")
	}
	if !caps.Has(types.CapVoteExtensions) {
		t.Error("missing CapVoteExtensions")
	}
	if !caps.Has(types.CapStateSync) {
		t.Error("missing CapStateSync")
	}
	if !caps.Has(types.CapSimulation) {
		t.Error("missing CapSimulation")
	}

	// Deposit via block execution.
	addr1 := dex.TestAddress(1)
	block := types.FinalizedBlock{
		Height: 1,
		Time:   types.TimeToTimestamp(time.Now()),
		Txs:    []types.Tx{dex.DepositTx(addr1, 1000, "ETH")},
	}
	outcome, err := client.ExecuteBlock(ctx, block)
	if err != nil {
		t.Fatalf("ExecuteBlock: %v", err)
	}
	if len(outcome.TxOutcomes) != 1 || outcome.TxOutcomes[0].Code != 0 {
		t.Fatalf("deposit failed: %+v", outcome.TxOutcomes)
	}
	if _, err := client.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// ProposalControl.
	pc := client.AsProposalControl()
	if pc == nil {
		t.Fatal("AsProposalControl returned nil")
	}
	built, err := pc.BuildProposal(ctx, types.ProposalContext{
		Height:     2,
		Time:       types.TimeToTimestamp(time.Now()),
		Proposer:   addr1,
		MaxTxBytes: 1048576,
	})
	if err != nil {
		t.Fatalf("BuildProposal: %v", err)
	}
	_ = built

	verdict, err := pc.VerifyProposal(ctx, types.ReceivedProposal{
		Height:   2,
		Time:     types.TimeToTimestamp(time.Now()),
		Proposer: addr1,
		Txs:      built.Txs,
	})
	if err != nil {
		t.Fatalf("VerifyProposal: %v", err)
	}
	if !verdict.Accept {
		t.Fatalf("proposal rejected: %s", verdict.RejectReason)
	}

	// VoteExtender.
	ve := client.AsVoteExtender()
	if ve == nil {
		t.Fatal("AsVoteExtender returned nil")
	}
	ext, err := ve.ExtendVote(ctx, types.VoteContext{Height: 1, Round: 0, BlockHash: types.Hash{0x01}})
	if err != nil {
		t.Fatalf("ExtendVote: %v", err)
	}
	_ = ext

	extVerdict, err := ve.VerifyExtension(ctx, types.ReceivedExtension{
		Height: 1, Round: 0,
		Validator: addr1,
		Extension: ext,
	})
	if err != nil {
		t.Fatalf("VerifyExtension: %v", err)
	}
	if extVerdict != types.ExtensionAccept {
		t.Fatalf("extension rejected")
	}

	// StateSync - AvailableSnapshots.
	ss := client.AsStateSync()
	if ss == nil {
		t.Fatal("AsStateSync returned nil")
	}
	snaps, err := ss.AvailableSnapshots(ctx)
	if err != nil {
		t.Fatalf("AvailableSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Height != 1 {
		t.Fatalf("snapshot height: got %d, want 1", snaps[0].Height)
	}

	// Simulator.
	sim := client.AsSimulator()
	if sim == nil {
		t.Fatal("AsSimulator returned nil")
	}
	simResult, err := sim.Simulate(ctx, dex.DepositTx(addr1, 500, "BTC"))
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if simResult.Code != 0 {
		t.Fatalf("Simulate failed: code=%d info=%s", simResult.Code, simResult.Info)
	}
}

func TestGRPC_Dex_ExportSnapshot(t *testing.T) {
	gs := bapigrpc.NewGRPCServer(dex.New())
	addr, cleanup := startServer(t, gs)
	defer cleanup()

	client := dial(t, addr)
	defer client.Close()

	ctx := context.Background()

	// Handshake + execute a block to create snapshot-able state.
	genesis := types.GenesisDoc{
		ChainID:       "dex-test",
		GenesisTime:   types.TimeToTimestamp(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		InitialHeight: 1,
		ConsensusParams: types.ConsensusParams{
			MaxBlockBytes:  1048576,
			MaxTxBytes:     65536,
			MaxEvidenceAge: types.DurationFromGo(24 * time.Hour),
		},
	}
	if _, err := client.Handshake(ctx, types.HandshakeRequest{Genesis: &genesis}); err != nil {
		t.Fatalf("Handshake: %v", err)
	}

	addr1 := dex.TestAddress(1)
	block := types.FinalizedBlock{
		Height: 1,
		Time:   types.TimeToTimestamp(time.Now()),
		Txs:    []types.Tx{dex.DepositTx(addr1, 1000, "ETH")},
	}
	if _, err := client.ExecuteBlock(ctx, block); err != nil {
		t.Fatalf("ExecuteBlock: %v", err)
	}
	if _, err := client.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Export snapshot.
	ss := client.AsStateSync()
	if ss == nil {
		t.Fatal("AsStateSync returned nil")
	}
	ch, _, err := ss.ExportSnapshot(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ExportSnapshot: %v", err)
	}

	var chunks []types.SnapshotChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	if len(chunks[0].Data) == 0 {
		t.Fatal("chunk data is empty")
	}
}

func TestGRPC_NilCapabilities(t *testing.T) {
	// Counter has no capabilities; capability accessors should return nil.
	gs := bapigrpc.NewGRPCServer(counter.New())
	addr, cleanup := startServer(t, gs)
	defer cleanup()

	client := dial(t, addr)
	defer client.Close()

	genesis := types.GenesisDoc{
		ChainID:       "test",
		GenesisTime:   types.TimeToTimestamp(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		InitialHeight: 1,
		ConsensusParams: types.ConsensusParams{
			MaxBlockBytes:  1048576,
			MaxTxBytes:     65536,
			MaxEvidenceAge: types.DurationFromGo(24 * time.Hour),
		},
	}
	if _, err := client.Handshake(context.Background(), types.HandshakeRequest{Genesis: &genesis}); err != nil {
		t.Fatalf("Handshake: %v", err)
	}

	if client.AsProposalControl() != nil {
		t.Error("counter should not have ProposalControl")
	}
	if client.AsVoteExtender() != nil {
		t.Error("counter should not have VoteExtender")
	}
	if client.AsStateSync() != nil {
		t.Error("counter should not have StateSync")
	}
	if client.AsSimulator() != nil {
		t.Error("counter should not have Simulator")
	}
}
