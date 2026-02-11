package bapigrpc

import (
	"bapi"
	"bapi/server"
	"bapi/types"
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"
)

// Compile-time interface check.
var _ bapi.Connection = (*Client)(nil)

// Client implements bapi.Connection for remote applications
// over gRPC using cramberry serialization. No protobuf types
// or conversion layer required.
type Client struct {
	cc    *grpc.ClientConn
	caps  types.Capabilities
	guard *server.LifecycleGuard
}

// Dial connects to a remote BAPI application.
func Dial(ctx context.Context, addr string, opts ...grpc.DialOption) (*Client, error) {
	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.ForceCodec(CramberryCodec{}),
	))
	cc, err := grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("bapi client: dial %s: %w", addr, err)
	}
	return &Client{
		cc:    cc,
		guard: server.NewLifecycleGuard(),
	}, nil
}

func (c *Client) Close() error {
	return c.cc.Close()
}

// --- Lifecycle ---

func (c *Client) Handshake(ctx context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error) {
	c.guard.AcquireHandshake()

	resp := new(types.HandshakeResponse)
	err := c.cc.Invoke(ctx, fullMethod("Handshake"), &req, resp)
	if err != nil {
		c.guard.FailHandshake()
		return types.HandshakeResponse{}, err
	}

	c.caps = resp.Capabilities
	c.guard.CompleteHandshake()
	return *resp, nil
}

func (c *Client) CheckTx(ctx context.Context, tx types.Tx, mctx types.MempoolContext) (types.GateVerdict, error) {
	c.guard.CheckConcurrent()

	req := &CheckTxRequest{Tx: tx, Context: mctx}
	resp := new(types.GateVerdict)
	if err := c.cc.Invoke(ctx, fullMethod("CheckTx"), req, resp); err != nil {
		return types.GateVerdict{}, err
	}
	return *resp, nil
}

func (c *Client) ExecuteBlock(ctx context.Context, block types.FinalizedBlock) (types.BlockOutcome, error) {
	c.guard.AcquireExecute()

	resp := new(types.BlockOutcome)
	err := c.cc.Invoke(ctx, fullMethod("ExecuteBlock"), &block, resp)
	if err != nil {
		c.guard.FailExecute()
		return types.BlockOutcome{}, err
	}

	c.guard.CompleteExecute()
	return *resp, nil
}

func (c *Client) Commit(ctx context.Context) (types.CommitResult, error) {
	c.guard.AcquireCommit()

	req := &CommitRequest{}
	resp := new(types.CommitResult)
	err := c.cc.Invoke(ctx, fullMethod("Commit"), req, resp)
	if err != nil {
		c.guard.CompleteCommit()
		return types.CommitResult{}, err
	}

	c.guard.CompleteCommit()
	return *resp, nil
}

func (c *Client) Query(ctx context.Context, req types.StateQuery) (types.StateQueryResult, error) {
	c.guard.CheckConcurrent()

	resp := new(types.StateQueryResult)
	if err := c.cc.Invoke(ctx, fullMethod("Query"), &req, resp); err != nil {
		return types.StateQueryResult{}, err
	}
	return *resp, nil
}

// --- Capability Accessors ---

func (c *Client) Capabilities() types.Capabilities { return c.caps }

func (c *Client) AsProposalControl() bapi.ProposalControl {
	if c.caps.Has(types.CapProposalControl) {
		return &clientProposalControl{c}
	}
	return nil
}

func (c *Client) AsVoteExtender() bapi.VoteExtender {
	if c.caps.Has(types.CapVoteExtensions) {
		return &clientVoteExtender{c}
	}
	return nil
}

func (c *Client) AsStateSync() bapi.StateSync {
	if c.caps.Has(types.CapStateSync) {
		return &clientStateSync{c}
	}
	return nil
}

func (c *Client) AsSimulator() bapi.Simulator {
	if c.caps.Has(types.CapSimulation) {
		return &clientSimulator{c}
	}
	return nil
}

// --- ProposalControl wrapper ---

type clientProposalControl struct{ c *Client }

func (w *clientProposalControl) BuildProposal(ctx context.Context, pctx types.ProposalContext) (types.BuiltProposal, error) {
	resp := new(types.BuiltProposal)
	if err := w.c.cc.Invoke(ctx, fullMethod("BuildProposal"), &pctx, resp); err != nil {
		return types.BuiltProposal{}, err
	}
	return *resp, nil
}

func (w *clientProposalControl) VerifyProposal(ctx context.Context, prop types.ReceivedProposal) (types.ProposalVerdict, error) {
	resp := new(types.ProposalVerdict)
	if err := w.c.cc.Invoke(ctx, fullMethod("VerifyProposal"), &prop, resp); err != nil {
		return types.ProposalVerdict{}, err
	}
	return *resp, nil
}

// --- VoteExtender wrapper ---

type clientVoteExtender struct{ c *Client }

func (w *clientVoteExtender) ExtendVote(ctx context.Context, vctx types.VoteContext) ([]byte, error) {
	resp := new(ExtendVoteResponse)
	if err := w.c.cc.Invoke(ctx, fullMethod("ExtendVote"), &vctx, resp); err != nil {
		return nil, err
	}
	return resp.Extension, nil
}

func (w *clientVoteExtender) VerifyExtension(ctx context.Context, ext types.ReceivedExtension) (types.ExtensionVerdict, error) {
	resp := new(ExtensionVerdictResponse)
	if err := w.c.cc.Invoke(ctx, fullMethod("VerifyExtension"), &ext, resp); err != nil {
		return types.ExtensionReject, err
	}
	return resp.Verdict, nil
}

// --- StateSync wrapper ---

type clientStateSync struct{ c *Client }

func (w *clientStateSync) AvailableSnapshots(ctx context.Context) ([]types.SnapshotDescriptor, error) {
	req := &AvailableSnapshotsRequest{}
	resp := new(AvailableSnapshotsResponse)
	if err := w.c.cc.Invoke(ctx, fullMethod("AvailableSnapshots"), req, resp); err != nil {
		return nil, err
	}
	return resp.Snapshots, nil
}

func (w *clientStateSync) ExportSnapshot(ctx context.Context, height uint64, format uint32) (<-chan types.SnapshotChunk, *types.SnapshotDescriptor, error) {
	req := &ExportSnapshotRequest{Height: height, Format: format}
	stream, err := w.c.cc.NewStream(ctx, &grpc.StreamDesc{
		StreamName:   "ExportSnapshot",
		ServerStreams: true,
	}, fullMethod("ExportSnapshot"))
	if err != nil {
		return nil, nil, err
	}
	if err := stream.SendMsg(req); err != nil {
		return nil, nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, nil, err
	}

	ch := make(chan types.SnapshotChunk)
	go func() {
		defer close(ch)
		for {
			chunk := new(types.SnapshotChunk)
			if err := stream.RecvMsg(chunk); err != nil {
				if err == io.EOF {
					return
				}
				return
			}
			ch <- *chunk
		}
	}()

	// Descriptor not available via streaming; caller should use
	// AvailableSnapshots first.
	return ch, nil, nil
}

func (w *clientStateSync) ImportSnapshot(ctx context.Context, desc types.SnapshotDescriptor, chunks <-chan types.SnapshotChunk) (types.ImportResult, error) {
	stream, err := w.c.cc.NewStream(ctx, &grpc.StreamDesc{
		StreamName:   "ImportSnapshot",
		ClientStreams: true,
	}, fullMethod("ImportSnapshot"))
	if err != nil {
		return types.ImportResult{}, err
	}

	// Send descriptor first.
	if err := stream.SendMsg(&ImportSnapshotMessage{Descriptor: &desc}); err != nil {
		return types.ImportResult{}, err
	}

	// Send chunks.
	for chunk := range chunks {
		c := chunk // capture for pointer
		if err := stream.SendMsg(&ImportSnapshotMessage{Chunk: &c}); err != nil {
			return types.ImportResult{}, err
		}
	}

	if err := stream.CloseSend(); err != nil {
		return types.ImportResult{}, err
	}

	result := new(types.ImportResult)
	if err := stream.RecvMsg(result); err != nil {
		return types.ImportResult{}, err
	}
	return *result, nil
}

// --- Simulator wrapper ---

type clientSimulator struct{ c *Client }

func (w *clientSimulator) Simulate(ctx context.Context, tx types.Tx) (types.TxOutcome, error) {
	req := &SimulateRequest{Tx: tx}
	resp := new(types.TxOutcome)
	if err := w.c.cc.Invoke(ctx, fullMethod("Simulate"), req, resp); err != nil {
		return types.TxOutcome{}, err
	}
	return *resp, nil
}
