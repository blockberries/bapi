package bapigrpc

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/blockberries/bapi"
	"github.com/blockberries/bapi/server"
	"github.com/blockberries/bapi/types"

	"google.golang.org/grpc"
)

// Compile-time interface check.
var _ BAPIServiceServer = (*GRPCServer)(nil)

// GRPCServer wraps a BAPI application as a gRPC server.
// No type conversion is needed — domain types are serialized
// directly via cramberry.
type GRPCServer struct {
	srv *server.Server
}

// NewGRPCServer creates a gRPC server wrapping the given application.
func NewGRPCServer(app bapi.Lifecycle) *GRPCServer {
	return &GRPCServer{
		srv: server.New(app),
	}
}

// Register adds the BAPI service to a gRPC server.
func (s *GRPCServer) Register(gs *grpc.Server) {
	RegisterBAPIServiceServer(gs, s)
}

// Serve starts the gRPC server on the given listener.
func (s *GRPCServer) Serve(lis net.Listener, opts ...grpc.ServerOption) error {
	gs := grpc.NewServer(opts...)
	s.Register(gs)
	return gs.Serve(lis)
}

// Stop gracefully stops the gRPC server.
func (s *GRPCServer) Stop(gs *grpc.Server) {
	gs.GracefulStop()
}

// Server returns the underlying server for advanced use.
func (s *GRPCServer) Server() *server.Server {
	return s.srv
}

// --- Lifecycle RPCs ---

func (s *GRPCServer) Handshake(ctx context.Context, req *types.HandshakeRequest) (*types.HandshakeResponse, error) {
	resp, err := s.srv.Handshake(ctx, *req)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *GRPCServer) CheckTx(ctx context.Context, req *CheckTxRequest) (*types.GateVerdict, error) {
	verdict, err := s.srv.CheckTx(ctx, req.Tx, req.Context)
	if err != nil {
		return nil, err
	}
	return &verdict, nil
}

func (s *GRPCServer) ExecuteBlock(ctx context.Context, block *types.FinalizedBlock) (*types.BlockOutcome, error) {
	outcome, err := s.srv.ExecuteBlock(ctx, *block)
	if err != nil {
		return nil, err
	}
	return &outcome, nil
}

func (s *GRPCServer) Commit(ctx context.Context, _ *CommitRequest) (*types.CommitResult, error) {
	result, err := s.srv.Commit(ctx)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *GRPCServer) Query(ctx context.Context, req *types.StateQuery) (*types.StateQueryResult, error) {
	result, err := s.srv.Query(ctx, *req)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// --- ProposalControl RPCs ---

func (s *GRPCServer) BuildProposal(ctx context.Context, pctx *types.ProposalContext) (*types.BuiltProposal, error) {
	proposal, err := s.srv.BuildProposal(ctx, *pctx)
	if err != nil {
		return nil, err
	}
	return &proposal, nil
}

func (s *GRPCServer) VerifyProposal(ctx context.Context, prop *types.ReceivedProposal) (*types.ProposalVerdict, error) {
	verdict, err := s.srv.VerifyProposal(ctx, *prop)
	if err != nil {
		return nil, err
	}
	return &verdict, nil
}

// --- VoteExtender RPCs ---

func (s *GRPCServer) ExtendVote(ctx context.Context, vctx *types.VoteContext) (*ExtendVoteResponse, error) {
	ext, err := s.srv.ExtendVote(ctx, *vctx)
	if err != nil {
		return nil, err
	}
	return &ExtendVoteResponse{Extension: ext}, nil
}

func (s *GRPCServer) VerifyExtension(ctx context.Context, ext *types.ReceivedExtension) (*ExtensionVerdictResponse, error) {
	verdict, err := s.srv.VerifyExtension(ctx, *ext)
	if err != nil {
		return nil, err
	}
	return &ExtensionVerdictResponse{Verdict: verdict}, nil
}

// --- StateSync RPCs ---

func (s *GRPCServer) AvailableSnapshots(ctx context.Context, _ *AvailableSnapshotsRequest) (*AvailableSnapshotsResponse, error) {
	snaps, err := s.srv.AvailableSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	return &AvailableSnapshotsResponse{Snapshots: snaps}, nil
}

func (s *GRPCServer) ExportSnapshot(req *ExportSnapshotRequest, stream grpc.ServerStream) error {
	ch, _, err := s.srv.ExportSnapshot(stream.Context(), req.Height, req.Format)
	if err != nil {
		return err
	}
	for chunk := range ch {
		if err := stream.SendMsg(&chunk); err != nil {
			return err
		}
	}
	return nil
}

func (s *GRPCServer) ImportSnapshot(stream grpc.ServerStream) error {
	// First message must be the descriptor.
	first := new(ImportSnapshotMessage)
	if err := stream.RecvMsg(first); err != nil {
		return err
	}
	if first.Descriptor == nil {
		return fmt.Errorf("github.com/blockberries/bapi grpc: first ImportSnapshot message must contain a descriptor")
	}

	desc := *first.Descriptor
	chunks := make(chan types.SnapshotChunk)

	// Read chunks in background.
	go func() {
		defer close(chunks)
		for {
			msg := new(ImportSnapshotMessage)
			if err := stream.RecvMsg(msg); err != nil {
				if err == io.EOF {
					return
				}
				return
			}
			if msg.Chunk != nil {
				chunks <- *msg.Chunk
			}
		}
	}()

	result, err := s.srv.ImportSnapshot(stream.Context(), desc, chunks)
	if err != nil {
		return err
	}

	return stream.SendMsg(&result)
}

// --- Simulator RPC ---

func (s *GRPCServer) Simulate(ctx context.Context, req *SimulateRequest) (*types.TxOutcome, error) {
	outcome, err := s.srv.Simulate(ctx, req.Tx)
	if err != nil {
		return nil, err
	}
	return &outcome, nil
}
