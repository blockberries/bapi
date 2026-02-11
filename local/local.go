// Package local provides a zero-copy, in-process BAPI connection.
//
// For applications compiled into the same binary as the consensus
// engine, this adapter wraps the application with lifecycle state
// machine enforcement and capability discovery — with no
// serialization overhead.
package local

import (
	"bapi"
	"bapi/server"
	"bapi/types"
	"context"
)

// Compile-time interface check.
var _ bapi.Connection = (*Connection)(nil)

// Connection wraps a local Lifecycle implementation with lifecycle
// enforcement and capability discovery.
type Connection struct {
	srv *server.Server
}

// NewConnection creates an in-process BAPI connection wrapping
// the given application.
func NewConnection(app bapi.Lifecycle) *Connection {
	return &Connection{srv: server.New(app)}
}

func (c *Connection) Handshake(ctx context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error) {
	return c.srv.Handshake(ctx, req)
}

func (c *Connection) CheckTx(ctx context.Context, tx types.Tx, mctx types.MempoolContext) (types.GateVerdict, error) {
	return c.srv.CheckTx(ctx, tx, mctx)
}

func (c *Connection) ExecuteBlock(ctx context.Context, block types.FinalizedBlock) (types.BlockOutcome, error) {
	return c.srv.ExecuteBlock(ctx, block)
}

func (c *Connection) Commit(ctx context.Context) (types.CommitResult, error) {
	return c.srv.Commit(ctx)
}

func (c *Connection) Query(ctx context.Context, req types.StateQuery) (types.StateQueryResult, error) {
	return c.srv.Query(ctx, req)
}

func (c *Connection) Capabilities() types.Capabilities {
	return c.srv.Capabilities()
}

func (c *Connection) AsProposalControl() bapi.ProposalControl {
	return c.srv.AsProposalControl()
}

func (c *Connection) AsVoteExtender() bapi.VoteExtender {
	return c.srv.AsVoteExtender()
}

func (c *Connection) AsStateSync() bapi.StateSync {
	return c.srv.AsStateSync()
}

func (c *Connection) AsSimulator() bapi.Simulator {
	return c.srv.AsSimulator()
}

func (c *Connection) Close() error { return nil }

// Server returns the underlying server for advanced use cases.
func (c *Connection) Server() *server.Server {
	return c.srv
}
