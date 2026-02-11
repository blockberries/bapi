package dex

import (
	"bapi"
	bapitest "bapi/testing"
	"bapi/types"
	"context"
	"encoding/binary"
	"testing"
)

func TestDex_Compliance(t *testing.T) {
	bapitest.RunComplianceSuite(t, func() bapi.Lifecycle {
		return New()
	})
}

func TestDex_Capabilities(t *testing.T) {
	h := bapitest.NewHarness(t, New())
	resp := h.GenesisDefault()

	if !resp.Capabilities.Has(types.CapProposalControl) {
		t.Error("expected CapProposalControl")
	}
	if !resp.Capabilities.Has(types.CapVoteExtensions) {
		t.Error("expected CapVoteExtensions")
	}
	if !resp.Capabilities.Has(types.CapStateSync) {
		t.Error("expected CapStateSync")
	}
	if !resp.Capabilities.Has(types.CapSimulation) {
		t.Error("expected CapSimulation")
	}
}

func TestDex_DepositAndWithdraw(t *testing.T) {
	app := New()
	h := bapitest.NewHarness(t, app)
	h.GenesisDefault()

	addr := TestAddress(1)

	// Deposit 1000 ETH.
	dep := DepositTx(addr, 1000, "ETH")
	outcome := h.ExecuteAndCommit(bapitest.MakeBlock(1, dep))
	if !outcome.TxOutcomes[0].OK() {
		t.Fatalf("deposit failed: %s", outcome.TxOutcomes[0].Info)
	}

	// Withdraw 300 ETH.
	wd := WithdrawTx(addr, 300, "ETH")
	outcome = h.ExecuteAndCommit(bapitest.MakeBlock(2, wd))
	if !outcome.TxOutcomes[0].OK() {
		t.Fatalf("withdraw failed: %s", outcome.TxOutcomes[0].Info)
	}

	// Check balance = 700.
	result := h.Query("/balance", append(addr[:], []byte("ETH")...))
	if result.Code != 0 {
		t.Fatalf("query failed: %s", result.Info)
	}
	bal := binary.BigEndian.Uint64(result.Value)
	if bal != 700 {
		t.Errorf("expected balance=700, got %d", bal)
	}
}

func TestDex_InsufficientBalance(t *testing.T) {
	app := New()
	h := bapitest.NewHarness(t, app)
	h.GenesisDefault()

	addr := TestAddress(1)

	// Deposit 100.
	h.ExecuteAndCommit(bapitest.MakeBlock(1, DepositTx(addr, 100, "ETH")))

	// Try to withdraw 200 — should fail.
	wd := WithdrawTx(addr, 200, "ETH")
	outcome := h.ExecuteAndCommit(bapitest.MakeBlock(2, wd))
	if outcome.TxOutcomes[0].OK() {
		t.Error("expected withdraw to fail with insufficient balance")
	}
}

func TestDex_LimitOrder(t *testing.T) {
	app := New()
	h := bapitest.NewHarness(t, app)
	h.GenesisDefault()

	addr := TestAddress(1)

	// Deposit 1000 ETH.
	h.ExecuteAndCommit(bapitest.MakeBlock(1, DepositTx(addr, 1000, "ETH")))

	// Place limit order: sell 500 ETH at price 2000 for ETH/USD pair.
	order := LimitOrderTx(addr, 500, 2000, "ETH/USD")
	outcome := h.ExecuteAndCommit(bapitest.MakeBlock(2, order))
	if !outcome.TxOutcomes[0].OK() {
		t.Fatalf("limit order failed: %s", outcome.TxOutcomes[0].Info)
	}

	// Check remaining balance = 500 (500 locked in order).
	result := h.Query("/balance", append(addr[:], []byte("ETH")...))
	bal := binary.BigEndian.Uint64(result.Value)
	if bal != 500 {
		t.Errorf("expected balance=500 after order, got %d", bal)
	}

	// Check order book.
	ordersResult := h.Query("/orders", nil)
	if ordersResult.Code != 0 {
		t.Fatalf("query /orders failed: %s", ordersResult.Info)
	}
}

func TestDex_OracleUpdate(t *testing.T) {
	app := New()
	h := bapitest.NewHarness(t, app)
	h.GenesisDefault()

	// Submit oracle update.
	oracleTx := OracleUpdateTx([]OraclePrice{
		{Pair: "ETH/USD", Price: 2500},
		{Pair: "BTC/USD", Price: 45000},
	})
	outcome := h.ExecuteAndCommit(bapitest.MakeBlock(1, oracleTx))
	if !outcome.TxOutcomes[0].OK() {
		t.Fatalf("oracle update failed: %s", outcome.TxOutcomes[0].Info)
	}

	// Query price.
	result := h.Query("/price", []byte("ETH/USD"))
	if result.Code != 0 {
		t.Fatalf("query /price failed: %s", result.Info)
	}
	price := binary.BigEndian.Uint64(result.Value)
	if price != 2500 {
		t.Errorf("expected price=2500, got %d", price)
	}
}

func TestDex_CheckTx_RejectsOracleFromMempool(t *testing.T) {
	app := New()
	h := bapitest.NewHarness(t, app)
	h.GenesisDefault()

	// Oracle updates should be rejected from the mempool.
	oracleTx := OracleUpdateTx([]OraclePrice{{Pair: "ETH/USD", Price: 2500}})
	h.MustRejectTx(oracleTx)

	// But deposits should be accepted.
	dep := DepositTx(TestAddress(1), 100, "ETH")
	h.MustAcceptTx(dep)
}

func TestDex_Simulate(t *testing.T) {
	app := New()
	h := bapitest.NewHarness(t, app)
	h.GenesisDefault()

	addr := TestAddress(1)

	// Deposit first.
	h.ExecuteAndCommit(bapitest.MakeBlock(1, DepositTx(addr, 1000, "ETH")))

	// Simulate a withdrawal.
	sim := app.AsSimulator()
	if sim == nil {
		// Access via the server.
		srv := h.Server()
		simIface := srv.AsSimulator()
		if simIface == nil {
			t.Fatal("expected Simulator capability")
		}
	}

	// Direct simulate on app.
	wd := WithdrawTx(addr, 500, "ETH")
	outcome, err := app.Simulate(context.TODO(), wd)
	if err != nil {
		t.Fatalf("simulate error: %v", err)
	}
	if !outcome.OK() {
		t.Fatalf("simulate failed: %s", outcome.Info)
	}

	// Balance should NOT have changed (simulation is non-mutating).
	result := h.Query("/balance", append(addr[:], []byte("ETH")...))
	bal := binary.BigEndian.Uint64(result.Value)
	if bal != 1000 {
		t.Errorf("simulation mutated state: expected balance=1000, got %d", bal)
	}
}

func TestDex_Deterministic(t *testing.T) {
	a1 := New()
	a2 := New()
	h1 := bapitest.NewHarness(t, a1)
	h2 := bapitest.NewHarness(t, a2)
	h1.GenesisDefault()
	h2.GenesisDefault()

	addr := TestAddress(1)
	txs := []types.Tx{
		DepositTx(addr, 1000, "ETH"),
		DepositTx(addr, 500, "USD"),
	}
	block := bapitest.MakeBlock(1, txs...)

	o1 := h1.ExecuteAndCommit(block)
	o2 := h2.ExecuteAndCommit(block)

	if o1.AppHash != o2.AppHash {
		t.Errorf("non-deterministic: %x != %x", o1.AppHash, o2.AppHash)
	}
}

// AsSimulator is a helper to access the Simulator from the app.
func (app *App) AsSimulator() bapi.Simulator {
	return app
}
