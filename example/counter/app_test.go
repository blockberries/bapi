package counter

import (
	"encoding/binary"
	"testing"

	"github.com/blockberries/bapi"
	bapitest "github.com/blockberries/bapi/testing"
	"github.com/blockberries/bapi/types"
)

func TestCounter_Compliance(t *testing.T) {
	bapitest.RunComplianceSuite(t, func() bapi.Lifecycle {
		return New()
	})
}

func TestCounter_Increment(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	h.GenesisDefault()

	// Increment by 5.
	tx := IncrementTx(5)
	outcome := h.ExecuteAndCommit(bapitest.MakeBlock(1, tx))

	if len(outcome.TxOutcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outcome.TxOutcomes))
	}
	if !outcome.TxOutcomes[0].OK() {
		t.Fatalf("tx failed: %s", outcome.TxOutcomes[0].Info)
	}

	// Query the count.
	result := h.Query("/count", nil)
	count := binary.BigEndian.Uint64(result.Value)
	if count != 5 {
		t.Errorf("expected count=5, got %d", count)
	}
}

func TestCounter_MultipleIncrements(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	h.GenesisDefault()

	// Block 1: increment by 3.
	h.ExecuteAndCommit(bapitest.MakeBlock(1, IncrementTx(3)))

	// Block 2: increment by 7.
	h.ExecuteAndCommit(bapitest.MakeBlock(2, IncrementTx(7)))

	// Query should return 10.
	result := h.Query("/count", nil)
	count := binary.BigEndian.Uint64(result.Value)
	if count != 10 {
		t.Errorf("expected count=10, got %d", count)
	}
}

func TestCounter_InvalidTx(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	h.GenesisDefault()

	// Bad tx: wrong length.
	h.MustRejectTx(types.Tx{0x01, 0x02, 0x03})

	// Good tx: valid.
	h.MustAcceptTx(IncrementTx(1))
}

func TestCounter_InvalidTxInBlock(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	h.GenesisDefault()

	// Block with a bad tx.
	bad := types.Tx{0x01, 0x02}
	good := IncrementTx(5)
	outcome := h.ExecuteAndCommit(bapitest.MakeBlock(1, bad, good))

	if len(outcome.TxOutcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(outcome.TxOutcomes))
	}
	if outcome.TxOutcomes[0].OK() {
		t.Error("expected first tx to fail")
	}
	if !outcome.TxOutcomes[1].OK() {
		t.Error("expected second tx to succeed")
	}

	// Count should be 5 (only the good tx counted).
	result := h.Query("/count", nil)
	count := binary.BigEndian.Uint64(result.Value)
	if count != 5 {
		t.Errorf("expected count=5, got %d", count)
	}
}

func TestCounter_EmptyBlocks(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	h.GenesisDefault()

	for i := uint64(1); i <= 5; i++ {
		outcome := h.ExecuteAndCommit(bapitest.MakeEmptyBlock(i))
		if outcome.AppHash == (types.AppHash{}) {
			t.Errorf("height %d: zero app hash", i)
		}
	}

	result := h.Query("/count", nil)
	count := binary.BigEndian.Uint64(result.Value)
	if count != 0 {
		t.Errorf("expected count=0 after empty blocks, got %d", count)
	}
}

func TestCounter_Deterministic(t *testing.T) {
	a1 := New()
	a2 := New()
	h1 := bapitest.NewHarness(t, a1)
	h2 := bapitest.NewHarness(t, a2)
	h1.GenesisDefault()
	h2.GenesisDefault()

	txs := []types.Tx{IncrementTx(3), IncrementTx(7)}
	block := bapitest.MakeBlock(1, txs...)

	o1 := h1.ExecuteAndCommit(block)
	o2 := h2.ExecuteAndCommit(block)

	if o1.AppHash != o2.AppHash {
		t.Errorf("non-deterministic: %x != %x", o1.AppHash, o2.AppHash)
	}
}

func TestCounter_Events(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	h.GenesisDefault()

	outcome := h.ExecuteAndCommit(bapitest.MakeBlock(1, IncrementTx(42)))

	txo := outcome.TxOutcomes[0]
	if len(txo.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(txo.Events))
	}
	e := txo.Events[0]
	if e.Kind != "increment" {
		t.Errorf("expected event kind 'increment', got %q", e.Kind)
	}
	if len(e.Attributes) < 2 {
		t.Fatalf("expected >= 2 attributes, got %d", len(e.Attributes))
	}
}

func TestCounter_QueryHeight(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	h.GenesisDefault()

	h.ExecuteAndCommit(bapitest.MakeBlock(1, IncrementTx(1)))
	h.ExecuteAndCommit(bapitest.MakeBlock(2, IncrementTx(2)))

	result := h.Query("/count", nil)
	if result.Height != 2 {
		t.Errorf("expected height=2, got %d", result.Height)
	}
}

func TestCounter_TxData(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	h.GenesisDefault()

	outcome := h.ExecuteAndCommit(bapitest.MakeBlock(1, IncrementTx(42)))

	txo := outcome.TxOutcomes[0]
	if txo.Data == nil {
		t.Fatal("expected non-nil Data in TxOutcome")
	}
	val := binary.BigEndian.Uint64(txo.Data)
	if val != 42 {
		t.Errorf("expected Data to encode 42, got %d", val)
	}
}
