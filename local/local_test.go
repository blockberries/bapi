package local

import (
	"bapi/example/counter"
	"bapi/types"
	"context"
	"encoding/binary"
	"testing"
)

func TestLocalConnection_FullCycle(t *testing.T) {
	app := counter.New()
	conn := NewConnection(app)
	defer conn.Close()

	// Handshake.
	_, err := conn.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &types.GenesisDoc{ChainID: "test"},
	})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}

	// Capabilities should be empty for counter app.
	if conn.Capabilities() != 0 {
		t.Errorf("expected no capabilities, got %s", conn.Capabilities())
	}

	// Optional interfaces should be nil.
	if conn.AsProposalControl() != nil {
		t.Error("expected nil ProposalControl")
	}
	if conn.AsVoteExtender() != nil {
		t.Error("expected nil VoteExtender")
	}
	if conn.AsStateSync() != nil {
		t.Error("expected nil StateSync")
	}
	if conn.AsSimulator() != nil {
		t.Error("expected nil Simulator")
	}

	// Execute and commit.
	tx := counter.IncrementTx(42)
	outcome, err := conn.ExecuteBlock(context.Background(), types.FinalizedBlock{
		Height: 1,
		Txs:    []types.Tx{tx},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !outcome.TxOutcomes[0].OK() {
		t.Fatalf("tx failed: %s", outcome.TxOutcomes[0].Info)
	}

	_, err = conn.Commit(context.Background())
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Query.
	result, err := conn.Query(context.Background(), types.StateQuery{
		Path: "/count",
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	count := binary.BigEndian.Uint64(result.Value)
	if count != 42 {
		t.Errorf("expected count=42, got %d", count)
	}
}

func TestLocalConnection_CheckTxConcurrent(t *testing.T) {
	conn := NewConnection(counter.New())

	_, err := conn.Handshake(context.Background(), types.HandshakeRequest{
		Genesis: &types.GenesisDoc{ChainID: "test"},
	})
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}

	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			tx := counter.IncrementTx(1)
			_, err := conn.CheckTx(context.Background(), tx, types.MempoolFirstSeen)
			if err != nil {
				t.Errorf("CheckTx error: %v", err)
			}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
