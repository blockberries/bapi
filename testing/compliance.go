package bapitest

import (
	"context"
	"sync"
	"testing"

	"github.com/blockberries/bapi"
	"github.com/blockberries/bapi/types"
)

// RunComplianceSuite runs a standard compliance test suite against
// a BAPI application to verify correct lifecycle behavior.
//
// The factory function should return a fresh application instance
// for each test.
func RunComplianceSuite(t *testing.T, factory func() bapi.Lifecycle) {
	t.Helper()

	t.Run("genesis_handshake", func(t *testing.T) {
		app := factory()
		h := NewHarness(t, app)
		resp := h.GenesisDefault()
		if resp.LastBlock != nil {
			t.Error("genesis handshake should return nil LastBlock")
		}
	})

	t.Run("genesis_returns_app_hash", func(t *testing.T) {
		app := factory()
		h := NewHarness(t, app)
		resp := h.GenesisDefault()
		if resp.AppHash == nil {
			t.Error("genesis handshake should return a non-nil AppHash")
		}
	})

	t.Run("execute_commit_cycle", func(t *testing.T) {
		app := factory()
		h := NewHarness(t, app)
		h.GenesisDefault()

		for i := uint64(1); i <= 5; i++ {
			outcome := h.ExecuteAndCommit(MakeEmptyBlock(i))
			if outcome.AppHash == (types.AppHash{}) {
				t.Errorf("height %d: zero app hash", i)
			}
		}
	})

	t.Run("empty_blocks_deterministic", func(t *testing.T) {
		// Execute same empty blocks on two instances, verify
		// identical AppHash.
		app1 := factory()
		h1 := NewHarness(t, app1)
		h1.GenesisDefault()

		app2 := factory()
		h2 := NewHarness(t, app2)
		h2.GenesisDefault()

		for i := uint64(1); i <= 3; i++ {
			block := MakeEmptyBlock(i)
			o1 := h1.ExecuteAndCommit(block)
			o2 := h2.ExecuteAndCommit(block)

			if o1.AppHash != o2.AppHash {
				t.Errorf("height %d: non-deterministic: %x != %x",
					i, o1.AppHash, o2.AppHash)
			}
		}
	})

	t.Run("deterministic_with_txs", func(t *testing.T) {
		app1 := factory()
		h1 := NewHarness(t, app1)
		h1.GenesisDefault()

		app2 := factory()
		h2 := NewHarness(t, app2)
		h2.GenesisDefault()

		tx := types.Tx([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
		block := MakeBlock(1, tx)

		o1 := h1.ExecuteAndCommit(block)
		o2 := h2.ExecuteAndCommit(block)

		if o1.AppHash != o2.AppHash {
			t.Errorf("non-deterministic with txs: %x != %x",
				o1.AppHash, o2.AppHash)
		}
		if len(o1.TxOutcomes) != len(o2.TxOutcomes) {
			t.Errorf("outcome count mismatch: %d != %d",
				len(o1.TxOutcomes), len(o2.TxOutcomes))
		}
	})

	t.Run("concurrent_checktx_after_handshake", func(t *testing.T) {
		app := factory()
		h := NewHarness(t, app)
		h.GenesisDefault()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tx := types.Tx([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
				_, err := h.Server().CheckTx(context.Background(), tx, types.MempoolFirstSeen)
				if err != nil {
					t.Errorf("concurrent CheckTx failed: %v", err)
				}
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent_query_after_handshake", func(t *testing.T) {
		app := factory()
		h := NewHarness(t, app)
		h.GenesisDefault()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := h.Server().Query(context.Background(), types.StateQuery{
					Path: "/test",
				})
				if err != nil {
					t.Errorf("concurrent Query failed: %v", err)
				}
			}()
		}
		wg.Wait()
	})

	t.Run("query_returns_height", func(t *testing.T) {
		app := factory()
		h := NewHarness(t, app)
		h.GenesisDefault()

		h.ExecuteAndCommit(MakeEmptyBlock(1))
		h.ExecuteAndCommit(MakeEmptyBlock(2))

		result := h.Query("/test", nil)
		if result.Height < 1 {
			t.Errorf("query height should be >= 1 after committing, got %d", result.Height)
		}
	})

	t.Run("tx_outcome_indices", func(t *testing.T) {
		app := factory()
		h := NewHarness(t, app)
		h.GenesisDefault()

		txs := []types.Tx{
			{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			{0x02, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			{0x03, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		}
		block := MakeBlock(1, txs...)
		outcome := h.ExecuteAndCommit(block)

		if len(outcome.TxOutcomes) != 3 {
			t.Fatalf("expected 3 tx outcomes, got %d", len(outcome.TxOutcomes))
		}
		for i, o := range outcome.TxOutcomes {
			if o.Index != uint32(i) {
				t.Errorf("tx %d: expected index %d, got %d", i, i, o.Index)
			}
		}
	})
}
