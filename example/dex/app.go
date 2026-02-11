// Package dex implements a full-featured DEX (decentralized exchange)
// BAPI application demonstrating all optional capabilities:
// ProposalControl, VoteExtensions, StateSync, and Simulation.
//
// Transaction format: a single prefix byte selects the type:
//
//	0x01 = deposit:      [1]prefix [20]address [8]amount [rest]denom
//	0x02 = withdraw:     [1]prefix [20]address [8]amount [rest]denom
//	0x03 = limit_order:  [1]prefix [20]address [8]amount [8]price [rest]pair
//	0x04 = oracle_update:[1]prefix [json]{"pair":"ETH/USD","price":123456},...
//
// Prices are stored as uint64 in hundredths (e.g., 123456 = $1234.56).
package dex

import (
	"bapi"
	"bapi/types"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Tx type prefix bytes.
const (
	TxDeposit      byte = 0x01
	TxWithdraw     byte = 0x02
	TxLimitOrder   byte = 0x03
	TxOracleUpdate byte = 0x04
)

// Compile-time interface checks.
var (
	_ bapi.Lifecycle       = (*App)(nil)
	_ bapi.ProposalControl = (*App)(nil)
	_ bapi.VoteExtender    = (*App)(nil)
	_ bapi.StateSync       = (*App)(nil)
	_ bapi.Simulator       = (*App)(nil)
)

// OraclePrice is a single price feed entry.
type OraclePrice struct {
	Pair  string `json:"pair"`
	Price uint64 `json:"price"`
}

// Order represents a limit order on the book.
type Order struct {
	Owner  types.ValidatorAddress `json:"owner"`
	Pair   string                `json:"pair"`
	Amount uint64                `json:"amount"`
	Price  uint64                `json:"price"`
}

// state holds the entire application state, designed for easy
// JSON serialization for state sync.
type state struct {
	Height   uint64                                  `json:"height"`
	Balances map[string]map[string]uint64            `json:"balances"` // address-hex -> denom -> amount
	Orders   []Order                                 `json:"orders"`
	Prices   map[string]uint64                       `json:"prices"` // pair -> price
}

func newState() *state {
	return &state{
		Balances: make(map[string]map[string]uint64),
		Orders:   nil,
		Prices:   make(map[string]uint64),
	}
}

func (s *state) clone() *state {
	c := &state{
		Height:   s.Height,
		Balances: make(map[string]map[string]uint64, len(s.Balances)),
		Orders:   make([]Order, len(s.Orders)),
		Prices:   make(map[string]uint64, len(s.Prices)),
	}
	for addr, denoms := range s.Balances {
		c.Balances[addr] = make(map[string]uint64, len(denoms))
		for d, v := range denoms {
			c.Balances[addr][d] = v
		}
	}
	copy(c.Orders, s.Orders)
	for p, v := range s.Prices {
		c.Prices[p] = v
	}
	return c
}

// appHash computes a deterministic SHA256 of the serialized state.
func (s *state) appHash() types.AppHash {
	data, _ := json.Marshal(s) // state is always serializable
	return types.AppHash(sha256.Sum256(data))
}

// App is a simple order-book DEX implementing all BAPI interfaces.
type App struct {
	mu      sync.RWMutex
	current *state
	staged  *state
}

// New creates a new DEX application with empty state.
func New() *App {
	return &App{
		current: newState(),
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func (app *App) Handshake(_ context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()

	caps := types.CapProposalControl | types.CapVoteExtensions | types.CapStateSync | types.CapSimulation

	if req.LastCommitted == nil {
		// Genesis.
		h := app.current.appHash()
		return types.HandshakeResponse{
			AppHash:      &h,
			Capabilities: caps,
		}, nil
	}
	// Restart.
	h := app.current.appHash()
	return types.HandshakeResponse{
		LastBlock: &types.BlockID{
			Height: app.current.Height,
		},
		AppHash:      &h,
		Capabilities: caps,
	}, nil
}

func (app *App) CheckTx(_ context.Context, tx types.Tx, _ types.MempoolContext) (types.GateVerdict, error) {
	if len(tx) == 0 {
		return types.GateVerdict{Code: 1, Info: "empty transaction"}, nil
	}
	switch tx[0] {
	case TxDeposit, TxWithdraw:
		if len(tx) < 30 { // 1 + 20 + 8 + at least 1 byte denom
			return types.GateVerdict{Code: 1, Info: "tx too short for deposit/withdraw"}, nil
		}
		return types.GateVerdict{Code: 0, Priority: 10, Sender: addrHex(tx[1:21])}, nil
	case TxLimitOrder:
		if len(tx) < 38 { // 1 + 20 + 8 + 8 + at least 1 byte pair
			return types.GateVerdict{Code: 1, Info: "tx too short for limit_order"}, nil
		}
		return types.GateVerdict{Code: 0, Priority: 5, Sender: addrHex(tx[1:21])}, nil
	case TxOracleUpdate:
		// Oracle updates are injected by BuildProposal; reject from mempool.
		return types.GateVerdict{Code: 1, Info: "oracle_update not accepted via mempool"}, nil
	default:
		return types.GateVerdict{Code: 1, Info: fmt.Sprintf("unknown tx type 0x%02x", tx[0])}, nil
	}
}

func (app *App) ExecuteBlock(_ context.Context, block types.FinalizedBlock) (types.BlockOutcome, error) {
	app.mu.RLock()
	s := app.current.clone()
	app.mu.RUnlock()

	s.Height = block.Height

	outcomes := make([]types.TxOutcome, len(block.Txs))
	var blockEvents []types.Event

	for i, tx := range block.Txs {
		outcome, events := executeTx(s, uint32(i), tx)
		outcomes[i] = outcome
		blockEvents = append(blockEvents, events...)
	}

	h := s.appHash()
	app.staged = s

	return types.BlockOutcome{
		TxOutcomes:  outcomes,
		BlockEvents: blockEvents,
		AppHash:     h,
	}, nil
}

func (app *App) Commit(_ context.Context) (types.CommitResult, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	app.current = app.staged
	app.staged = nil

	retain := uint64(0)
	if app.current.Height > 100 {
		retain = app.current.Height - 100
	}
	return types.CommitResult{RetainHeight: retain}, nil
}

func (app *App) Query(_ context.Context, req types.StateQuery) (types.StateQueryResult, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()

	switch string(req.Path) {
	case "/balance":
		// Data = [20]address + denom
		if len(req.Data) < 21 {
			return types.StateQueryResult{Code: 1, Info: "data must be [20]address + denom"}, nil
		}
		addr := addrHex(req.Data[:20])
		denom := string(req.Data[20:])
		bal := app.current.Balances[addr][denom]
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, bal)
		return types.StateQueryResult{
			Code:   0,
			Key:    req.Data,
			Value:  buf,
			Height: app.current.Height,
		}, nil

	case "/price":
		pair := string(req.Data)
		price, ok := app.current.Prices[pair]
		if !ok {
			return types.StateQueryResult{Code: 1, Info: "unknown pair", Height: app.current.Height}, nil
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, price)
		return types.StateQueryResult{
			Code:   0,
			Key:    req.Data,
			Value:  buf,
			Height: app.current.Height,
		}, nil

	case "/orders":
		data, _ := json.Marshal(app.current.Orders)
		return types.StateQueryResult{
			Code:   0,
			Value:  data,
			Height: app.current.Height,
		}, nil

	default:
		return types.StateQueryResult{Code: 1, Info: "unknown query path", Height: app.current.Height}, nil
	}
}

// ---------------------------------------------------------------------------
// ProposalControl
// ---------------------------------------------------------------------------

func (app *App) BuildProposal(_ context.Context, pctx types.ProposalContext) (types.BuiltProposal, error) {
	var txs []types.Tx
	totalBytes := uint64(0)

	// If we have vote extensions, aggregate oracle prices and inject an
	// oracle_update transaction as the first tx.
	if len(pctx.VoteExtensions) > 0 {
		oracleTx := buildOracleTxFromExtensions(pctx.VoteExtensions)
		if oracleTx != nil {
			txSize := uint64(len(oracleTx))
			if txSize <= pctx.MaxTxBytes {
				txs = append(txs, oracleTx)
				totalBytes += txSize
			}
		}
	}

	// Fill remaining space with mempool txs.
	for _, tx := range pctx.MempoolTxs {
		txSize := uint64(len(tx))
		if totalBytes+txSize > pctx.MaxTxBytes {
			continue
		}
		// Skip oracle updates from the mempool.
		if len(tx) > 0 && tx[0] == TxOracleUpdate {
			continue
		}
		txs = append(txs, tx)
		totalBytes += txSize
	}

	return types.BuiltProposal{Txs: txs}, nil
}

func (app *App) VerifyProposal(_ context.Context, proposal types.ReceivedProposal) (types.ProposalVerdict, error) {
	hasExtensions := len(proposal.VoteExtensions) > 0

	if len(proposal.Txs) == 0 {
		return types.ProposalVerdict{Accept: true}, nil
	}

	firstTx := proposal.Txs[0]
	firstIsOracle := len(firstTx) > 0 && firstTx[0] == TxOracleUpdate

	if hasExtensions && !firstIsOracle {
		// Extensions were available but proposer did not inject an oracle tx.
		// This is acceptable -- the proposer may not have had valid prices.
		return types.ProposalVerdict{Accept: true}, nil
	}

	if firstIsOracle {
		// Validate the oracle update deserializes correctly.
		var prices []OraclePrice
		if err := json.Unmarshal(firstTx[1:], &prices); err != nil {
			return types.ProposalVerdict{
				Accept:       false,
				RejectReason: fmt.Sprintf("invalid oracle_update tx: %v", err),
			}, nil
		}
		for _, p := range prices {
			if p.Pair == "" || p.Price == 0 {
				return types.ProposalVerdict{
					Accept:       false,
					RejectReason: "oracle_update contains empty pair or zero price",
				}, nil
			}
		}
	}

	// Verify no other tx is an oracle update.
	for i := 1; i < len(proposal.Txs); i++ {
		if len(proposal.Txs[i]) > 0 && proposal.Txs[i][0] == TxOracleUpdate {
			return types.ProposalVerdict{
				Accept:       false,
				RejectReason: "oracle_update tx must only appear at index 0",
			}, nil
		}
	}

	return types.ProposalVerdict{Accept: true}, nil
}

// ---------------------------------------------------------------------------
// VoteExtender
// ---------------------------------------------------------------------------

func (app *App) ExtendVote(_ context.Context, _ types.VoteContext) ([]byte, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()

	// Publish our current oracle prices as the vote extension.
	var prices []OraclePrice
	for pair, price := range app.current.Prices {
		prices = append(prices, OraclePrice{Pair: pair, Price: price})
	}
	// Sort for determinism.
	sort.Slice(prices, func(i, j int) bool {
		return prices[i].Pair < prices[j].Pair
	})

	data, err := json.Marshal(prices)
	if err != nil {
		return nil, fmt.Errorf("marshal oracle prices: %w", err)
	}
	return data, nil
}

func (app *App) VerifyExtension(_ context.Context, ext types.ReceivedExtension) (types.ExtensionVerdict, error) {
	if len(ext.Extension) == 0 {
		// Empty extension is acceptable (validator has no prices yet).
		return types.ExtensionAccept, nil
	}
	var prices []OraclePrice
	if err := json.Unmarshal(ext.Extension, &prices); err != nil {
		return types.ExtensionReject, nil
	}
	for _, p := range prices {
		if p.Pair == "" {
			return types.ExtensionReject, nil
		}
	}
	return types.ExtensionAccept, nil
}

// ---------------------------------------------------------------------------
// StateSync
// ---------------------------------------------------------------------------

const snapshotFormat uint32 = 1
const snapshotChunkSize = 64 * 1024 // 64 KiB per chunk

func (app *App) AvailableSnapshots(_ context.Context) ([]types.SnapshotDescriptor, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()

	if app.current.Height == 0 {
		return nil, nil
	}

	data, err := json.Marshal(app.current)
	if err != nil {
		return nil, fmt.Errorf("marshal state: %w", err)
	}

	hash := sha256.Sum256(data)
	nChunks := (uint32(len(data)) + snapshotChunkSize - 1) / snapshotChunkSize

	return []types.SnapshotDescriptor{{
		Height: app.current.Height,
		Format: snapshotFormat,
		Chunks: nChunks,
		Hash:   types.Hash(hash),
	}}, nil
}

func (app *App) ExportSnapshot(_ context.Context, height uint64, format uint32) (<-chan types.SnapshotChunk, *types.SnapshotDescriptor, error) {
	app.mu.RLock()
	defer app.mu.RUnlock()

	if format != snapshotFormat {
		return nil, nil, fmt.Errorf("unsupported snapshot format %d", format)
	}
	if app.current.Height != height {
		return nil, nil, fmt.Errorf("snapshot at height %d not available (current: %d)", height, app.current.Height)
	}

	data, err := json.Marshal(app.current)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal state: %w", err)
	}

	hash := sha256.Sum256(data)
	nChunks := (uint32(len(data)) + snapshotChunkSize - 1) / snapshotChunkSize

	desc := &types.SnapshotDescriptor{
		Height: height,
		Format: snapshotFormat,
		Chunks: nChunks,
		Hash:   types.Hash(hash),
	}

	ch := make(chan types.SnapshotChunk, nChunks)
	go func() {
		defer close(ch)
		for i := uint32(0); i < nChunks; i++ {
			start := i * snapshotChunkSize
			end := start + snapshotChunkSize
			if int(end) > len(data) {
				end = uint32(len(data))
			}
			ch <- types.SnapshotChunk{
				Index: i,
				Data:  data[start:end],
			}
		}
	}()

	return ch, desc, nil
}

func (app *App) ImportSnapshot(_ context.Context, descriptor types.SnapshotDescriptor, chunks <-chan types.SnapshotChunk) (types.ImportResult, error) {
	if descriptor.Format != snapshotFormat {
		return types.ImportResult{
			Status: types.ImportReject,
			Reason: fmt.Sprintf("unsupported format %d", descriptor.Format),
		}, nil
	}

	// Collect chunks in order.
	received := make(map[uint32][]byte)
	for chunk := range chunks {
		received[chunk.Index] = chunk.Data
	}

	// Verify we got all chunks.
	if uint32(len(received)) != descriptor.Chunks {
		var missing []uint32
		for i := uint32(0); i < descriptor.Chunks; i++ {
			if _, ok := received[i]; !ok {
				missing = append(missing, i)
			}
		}
		return types.ImportResult{
			Status:       types.ImportRetryChunks,
			RetryIndices: missing,
		}, nil
	}

	// Reassemble.
	var full []byte
	for i := uint32(0); i < descriptor.Chunks; i++ {
		full = append(full, received[i]...)
	}

	// Verify hash.
	hash := sha256.Sum256(full)
	if types.Hash(hash) != descriptor.Hash {
		return types.ImportResult{
			Status: types.ImportReject,
			Reason: "snapshot hash mismatch",
		}, nil
	}

	// Deserialize.
	s := newState()
	if err := json.Unmarshal(full, s); err != nil {
		return types.ImportResult{
			Status: types.ImportReject,
			Reason: fmt.Sprintf("unmarshal state: %v", err),
		}, nil
	}

	app.mu.Lock()
	app.current = s
	app.mu.Unlock()

	appHash := s.appHash()
	return types.ImportResult{
		Status:  types.ImportOK,
		AppHash: &appHash,
	}, nil
}

// ---------------------------------------------------------------------------
// Simulator
// ---------------------------------------------------------------------------

func (app *App) Simulate(_ context.Context, tx types.Tx) (types.TxOutcome, error) {
	app.mu.RLock()
	s := app.current.clone()
	app.mu.RUnlock()

	outcome, _ := executeTx(s, 0, tx)
	return outcome, nil
}

// ---------------------------------------------------------------------------
// Internal: transaction execution
// ---------------------------------------------------------------------------

func executeTx(s *state, index uint32, tx types.Tx) (types.TxOutcome, []types.Event) {
	if len(tx) == 0 {
		return types.TxOutcome{Index: index, Code: 1, Info: "empty tx"}, nil
	}

	switch tx[0] {
	case TxDeposit:
		return executeDeposit(s, index, tx)
	case TxWithdraw:
		return executeWithdraw(s, index, tx)
	case TxLimitOrder:
		return executeLimitOrder(s, index, tx)
	case TxOracleUpdate:
		return executeOracleUpdate(s, index, tx)
	default:
		return types.TxOutcome{
			Index: index,
			Code:  1,
			Info:  fmt.Sprintf("unknown tx type 0x%02x", tx[0]),
		}, nil
	}
}

func executeDeposit(s *state, index uint32, tx types.Tx) (types.TxOutcome, []types.Event) {
	if len(tx) < 30 {
		return types.TxOutcome{Index: index, Code: 1, Info: "deposit tx too short"}, nil
	}

	addr := addrHex(tx[1:21])
	amount := binary.BigEndian.Uint64(tx[21:29])
	denom := string(tx[29:])

	if amount == 0 {
		return types.TxOutcome{Index: index, Code: 1, Info: "zero deposit"}, nil
	}

	if s.Balances[addr] == nil {
		s.Balances[addr] = make(map[string]uint64)
	}
	s.Balances[addr][denom] += amount

	return types.TxOutcome{
		Index: index,
		Code:  0,
		Data:  encodeUint64(s.Balances[addr][denom]),
		Events: []types.Event{{
			Kind: "deposit",
			Attributes: []types.EventAttribute{
				{Key: "address", Value: addr, Index: true},
				{Key: "denom", Value: denom, Index: true},
				{Key: "amount", Value: fmt.Sprintf("%d", amount), Index: true},
			},
		}},
	}, nil
}

func executeWithdraw(s *state, index uint32, tx types.Tx) (types.TxOutcome, []types.Event) {
	if len(tx) < 30 {
		return types.TxOutcome{Index: index, Code: 1, Info: "withdraw tx too short"}, nil
	}

	addr := addrHex(tx[1:21])
	amount := binary.BigEndian.Uint64(tx[21:29])
	denom := string(tx[29:])

	if amount == 0 {
		return types.TxOutcome{Index: index, Code: 1, Info: "zero withdrawal"}, nil
	}

	bal := s.Balances[addr][denom]
	if bal < amount {
		return types.TxOutcome{
			Index: index,
			Code:  2,
			Info:  fmt.Sprintf("insufficient balance: have %d, need %d", bal, amount),
		}, nil
	}

	s.Balances[addr][denom] -= amount
	if s.Balances[addr][denom] == 0 {
		delete(s.Balances[addr], denom)
		if len(s.Balances[addr]) == 0 {
			delete(s.Balances, addr)
		}
	}

	return types.TxOutcome{
		Index: index,
		Code:  0,
		Data:  encodeUint64(s.Balances[addr][denom]),
		Events: []types.Event{{
			Kind: "withdraw",
			Attributes: []types.EventAttribute{
				{Key: "address", Value: addr, Index: true},
				{Key: "denom", Value: denom, Index: true},
				{Key: "amount", Value: fmt.Sprintf("%d", amount), Index: true},
			},
		}},
	}, nil
}

func executeLimitOrder(s *state, index uint32, tx types.Tx) (types.TxOutcome, []types.Event) {
	if len(tx) < 38 {
		return types.TxOutcome{Index: index, Code: 1, Info: "limit_order tx too short"}, nil
	}

	var owner types.ValidatorAddress
	copy(owner[:], tx[1:21])
	addr := addrHex(tx[1:21])
	amount := binary.BigEndian.Uint64(tx[21:29])
	price := binary.BigEndian.Uint64(tx[29:37])
	pair := string(tx[37:])

	if amount == 0 || price == 0 {
		return types.TxOutcome{Index: index, Code: 1, Info: "zero amount or price"}, nil
	}

	// The base denom is the part before "/" in the pair (e.g., "ETH" in "ETH/USD").
	baseDenom := pairBase(pair)

	// Check balance for the base denom.
	bal := s.Balances[addr][baseDenom]
	if bal < amount {
		return types.TxOutcome{
			Index: index,
			Code:  2,
			Info:  fmt.Sprintf("insufficient %s balance: have %d, need %d", baseDenom, bal, amount),
		}, nil
	}

	// Lock the funds.
	s.Balances[addr][baseDenom] -= amount

	// Add to order book.
	order := Order{
		Owner:  owner,
		Pair:   pair,
		Amount: amount,
		Price:  price,
	}
	s.Orders = append(s.Orders, order)

	return types.TxOutcome{
		Index: index,
		Code:  0,
		Data:  encodeUint64(uint64(len(s.Orders) - 1)), // order index
		Events: []types.Event{{
			Kind: "limit_order",
			Attributes: []types.EventAttribute{
				{Key: "address", Value: addr, Index: true},
				{Key: "pair", Value: pair, Index: true},
				{Key: "amount", Value: fmt.Sprintf("%d", amount), Index: false},
				{Key: "price", Value: fmt.Sprintf("%d", price), Index: false},
			},
		}},
	}, nil
}

func executeOracleUpdate(s *state, index uint32, tx types.Tx) (types.TxOutcome, []types.Event) {
	if len(tx) < 2 {
		return types.TxOutcome{Index: index, Code: 1, Info: "oracle_update tx too short"}, nil
	}

	var prices []OraclePrice
	if err := json.Unmarshal(tx[1:], &prices); err != nil {
		return types.TxOutcome{
			Index: index,
			Code:  1,
			Info:  fmt.Sprintf("invalid oracle JSON: %v", err),
		}, nil
	}

	var events []types.Event
	for _, p := range prices {
		if p.Pair == "" || p.Price == 0 {
			continue
		}
		s.Prices[p.Pair] = p.Price
		events = append(events, types.Event{
			Kind: "oracle_update",
			Attributes: []types.EventAttribute{
				{Key: "pair", Value: p.Pair, Index: true},
				{Key: "price", Value: fmt.Sprintf("%d", p.Price), Index: true},
			},
		})
	}

	return types.TxOutcome{
		Index:  index,
		Code:   0,
		Events: events,
	}, events
}

// ---------------------------------------------------------------------------
// Oracle aggregation helpers
// ---------------------------------------------------------------------------

// buildOracleTxFromExtensions aggregates price feeds from vote extensions
// by computing the median price for each pair, then encodes the result as
// an oracle_update transaction.
func buildOracleTxFromExtensions(exts []types.CommittedVoteExtension) types.Tx {
	// Collect all prices per pair.
	pairPrices := make(map[string][]uint64)

	for _, ext := range exts {
		if len(ext.Extension) == 0 {
			continue
		}
		var prices []OraclePrice
		if err := json.Unmarshal(ext.Extension, &prices); err != nil {
			continue
		}
		for _, p := range prices {
			if p.Pair != "" && p.Price > 0 {
				pairPrices[p.Pair] = append(pairPrices[p.Pair], p.Price)
			}
		}
	}

	if len(pairPrices) == 0 {
		return nil
	}

	// Compute medians.
	var aggregated []OraclePrice
	for pair, vals := range pairPrices {
		sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
		median := vals[len(vals)/2]
		aggregated = append(aggregated, OraclePrice{Pair: pair, Price: median})
	}
	// Sort for determinism.
	sort.Slice(aggregated, func(i, j int) bool {
		return aggregated[i].Pair < aggregated[j].Pair
	})

	payload, err := json.Marshal(aggregated)
	if err != nil {
		return nil
	}
	tx := make(types.Tx, 1+len(payload))
	tx[0] = TxOracleUpdate
	copy(tx[1:], payload)
	return tx
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func addrHex(b []byte) string {
	return fmt.Sprintf("%x", b)
}

func encodeUint64(v uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	return buf
}

// pairBase extracts the base denomination from a trading pair string.
// For "ETH/USD" it returns "ETH". If no "/" is found, returns the full string.
func pairBase(pair string) string {
	for i := 0; i < len(pair); i++ {
		if pair[i] == '/' {
			return pair[:i]
		}
	}
	return pair
}

// ---------------------------------------------------------------------------
// Test transaction builders
// ---------------------------------------------------------------------------

// DepositTx creates a deposit transaction.
func DepositTx(addr types.ValidatorAddress, amount uint64, denom string) types.Tx {
	tx := make(types.Tx, 1+20+8+len(denom))
	tx[0] = TxDeposit
	copy(tx[1:21], addr[:])
	binary.BigEndian.PutUint64(tx[21:29], amount)
	copy(tx[29:], denom)
	return tx
}

// WithdrawTx creates a withdrawal transaction.
func WithdrawTx(addr types.ValidatorAddress, amount uint64, denom string) types.Tx {
	tx := make(types.Tx, 1+20+8+len(denom))
	tx[0] = TxWithdraw
	copy(tx[1:21], addr[:])
	binary.BigEndian.PutUint64(tx[21:29], amount)
	copy(tx[29:], denom)
	return tx
}

// LimitOrderTx creates a limit order transaction.
func LimitOrderTx(addr types.ValidatorAddress, amount, price uint64, pair string) types.Tx {
	tx := make(types.Tx, 1+20+8+8+len(pair))
	tx[0] = TxLimitOrder
	copy(tx[1:21], addr[:])
	binary.BigEndian.PutUint64(tx[21:29], amount)
	binary.BigEndian.PutUint64(tx[29:37], price)
	copy(tx[37:], pair)
	return tx
}

// OracleUpdateTx creates an oracle update transaction from prices.
func OracleUpdateTx(prices []OraclePrice) types.Tx {
	payload, _ := json.Marshal(prices)
	tx := make(types.Tx, 1+len(payload))
	tx[0] = TxOracleUpdate
	copy(tx[1:], payload)
	return tx
}

// TestAddress creates a deterministic test address from an index.
func TestAddress(n byte) types.ValidatorAddress {
	var addr types.ValidatorAddress
	addr[0] = n
	addr[19] = n
	return addr
}
