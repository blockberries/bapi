// Package counter implements a minimal BAPI application that
// counts transactions. It demonstrates the core Lifecycle
// interface with no optional capabilities.
//
// Transaction format: 8 bytes, big-endian uint64 increment value.
package counter

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/blockberries/bapi"
	"github.com/blockberries/bapi/types"
)

// Compile-time interface check.
var _ bapi.Lifecycle = (*App)(nil)

// App is a minimal BAPI application that counts transactions.
type App struct {
	mu      sync.RWMutex
	count   uint64
	appHash types.AppHash
	height  uint64

	// Staging area (between ExecuteBlock and Commit).
	staged struct {
		count   uint64
		appHash types.AppHash
		height  uint64
	}
}

// New creates a new counter application.
func New() *App {
	return &App{
		appHash: computeHash(0),
	}
}

func (app *App) Handshake(_ context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error) {
	if req.LastCommitted == nil {
		// Genesis: initialize to zero.
		h := computeHash(0)
		return types.HandshakeResponse{
			AppHash:      &h,
			Capabilities: 0,
		}, nil
	}
	// Restart: report persisted state.
	app.mu.RLock()
	defer app.mu.RUnlock()
	return types.HandshakeResponse{
		LastBlock: &types.BlockID{
			Height: app.height,
		},
		AppHash:      &app.appHash,
		Capabilities: 0,
	}, nil
}

func (app *App) CheckTx(_ context.Context, tx types.Tx, _ types.MempoolContext) (types.GateVerdict, error) {
	if err := validateTx(tx); err != nil {
		return types.GateVerdict{
			Code: 1,
			Info: err.Error(),
		}, nil
	}
	return types.GateVerdict{
		Code:     0,
		Priority: 0,
	}, nil
}

func (app *App) ExecuteBlock(_ context.Context, block types.FinalizedBlock) (types.BlockOutcome, error) {
	app.mu.RLock()
	newCount := app.count
	app.mu.RUnlock()

	outcomes := make([]types.TxOutcome, len(block.Txs))

	for i, tx := range block.Txs {
		if err := validateTx(tx); err != nil {
			outcomes[i] = types.TxOutcome{
				Index: uint32(i),
				Code:  1,
				Info:  err.Error(),
			}
			continue
		}

		inc := binary.BigEndian.Uint64(tx)
		newCount += inc

		countBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(countBytes, newCount)

		outcomes[i] = types.TxOutcome{
			Index: uint32(i),
			Code:  0,
			Data:  countBytes,
			Events: []types.Event{{
				Kind: "increment",
				Attributes: []types.EventAttribute{
					{Key: "by", Value: fmt.Sprintf("%d", inc), Index: true},
					{Key: "total", Value: fmt.Sprintf("%d", newCount), Index: true},
				},
			}},
		}
	}

	newHash := computeHash(newCount)

	// Stage changes (not persisted until Commit).
	app.staged.count = newCount
	app.staged.appHash = newHash
	app.staged.height = block.Height

	return types.BlockOutcome{
		TxOutcomes: outcomes,
		AppHash:    newHash,
	}, nil
}

func (app *App) Commit(_ context.Context) (types.CommitResult, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	app.count = app.staged.count
	app.appHash = app.staged.appHash
	app.height = app.staged.height

	return types.CommitResult{RetainHeight: 0}, nil
}

func (app *App) Query(_ context.Context, _ types.StateQuery) (types.StateQueryResult, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, app.count)

	return types.StateQueryResult{
		Code:   0,
		Key:    []byte("count"),
		Value:  buf,
		Height: app.height,
	}, nil
}

// Count returns the current committed counter value.
func (app *App) Count() uint64 {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.count
}

func validateTx(tx types.Tx) error {
	if len(tx) != 8 {
		return fmt.Errorf("tx must be 8 bytes, got %d", len(tx))
	}
	return nil
}

func computeHash(count uint64) types.AppHash {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, count)
	return types.AppHash(sha256.Sum256(buf))
}

// IncrementTx creates a transaction that increments by the
// given value.
func IncrementTx(n uint64) types.Tx {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, n)
	return buf
}
