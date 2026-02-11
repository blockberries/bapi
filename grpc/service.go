package bapigrpc

import (
	"context"
	"fmt"

	"github.com/blockberries/bapi/types"

	"google.golang.org/grpc"
)

const serviceName = "github.com/blockberries/bapi.v1.BAPIService"

// BAPIServiceServer is the server-side interface for the BAPI gRPC service.
type BAPIServiceServer interface {
	Handshake(context.Context, *types.HandshakeRequest) (*types.HandshakeResponse, error)
	CheckTx(context.Context, *CheckTxRequest) (*types.GateVerdict, error)
	ExecuteBlock(context.Context, *types.FinalizedBlock) (*types.BlockOutcome, error)
	Commit(context.Context, *CommitRequest) (*types.CommitResult, error)
	Query(context.Context, *types.StateQuery) (*types.StateQueryResult, error)
	BuildProposal(context.Context, *types.ProposalContext) (*types.BuiltProposal, error)
	VerifyProposal(context.Context, *types.ReceivedProposal) (*types.ProposalVerdict, error)
	ExtendVote(context.Context, *types.VoteContext) (*ExtendVoteResponse, error)
	VerifyExtension(context.Context, *types.ReceivedExtension) (*ExtensionVerdictResponse, error)
	AvailableSnapshots(context.Context, *AvailableSnapshotsRequest) (*AvailableSnapshotsResponse, error)
	ExportSnapshot(*ExportSnapshotRequest, grpc.ServerStream) error
	ImportSnapshot(grpc.ServerStream) error
	Simulate(context.Context, *SimulateRequest) (*types.TxOutcome, error)
}

// RegisterBAPIServiceServer registers the BAPIServiceServer on a gRPC server.
func RegisterBAPIServiceServer(s *grpc.Server, srv BAPIServiceServer) {
	s.RegisterService(&serviceDesc, srv)
}

// --- Handler functions ---

func handlerHandshake(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(types.HandshakeRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).Handshake(ctx, req)
}

func handlerCheckTx(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(CheckTxRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).CheckTx(ctx, req)
}

func handlerExecuteBlock(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(types.FinalizedBlock)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).ExecuteBlock(ctx, req)
}

func handlerCommit(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(CommitRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).Commit(ctx, req)
}

func handlerQuery(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(types.StateQuery)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).Query(ctx, req)
}

func handlerBuildProposal(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(types.ProposalContext)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).BuildProposal(ctx, req)
}

func handlerVerifyProposal(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(types.ReceivedProposal)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).VerifyProposal(ctx, req)
}

func handlerExtendVote(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(types.VoteContext)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).ExtendVote(ctx, req)
}

func handlerVerifyExtension(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(types.ReceivedExtension)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).VerifyExtension(ctx, req)
}

func handlerAvailableSnapshots(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(AvailableSnapshotsRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).AvailableSnapshots(ctx, req)
}

func handlerExportSnapshot(srv any, stream grpc.ServerStream) error {
	req := new(ExportSnapshotRequest)
	if err := stream.RecvMsg(req); err != nil {
		return err
	}
	return srv.(BAPIServiceServer).ExportSnapshot(req, stream)
}

func handlerImportSnapshot(srv any, stream grpc.ServerStream) error {
	return srv.(BAPIServiceServer).ImportSnapshot(stream)
}

func handlerSimulate(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	req := new(SimulateRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(BAPIServiceServer).Simulate(ctx, req)
}

// fullMethod builds the full gRPC method path.
func fullMethod(method string) string {
	return fmt.Sprintf("/%s/%s", serviceName, method)
}

// serviceDesc is the manual gRPC service descriptor for BAPI.
var serviceDesc = grpc.ServiceDesc{
	ServiceName: serviceName,
	HandlerType: (*BAPIServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Handshake", Handler: handlerHandshake},
		{MethodName: "CheckTx", Handler: handlerCheckTx},
		{MethodName: "ExecuteBlock", Handler: handlerExecuteBlock},
		{MethodName: "Commit", Handler: handlerCommit},
		{MethodName: "Query", Handler: handlerQuery},
		{MethodName: "BuildProposal", Handler: handlerBuildProposal},
		{MethodName: "VerifyProposal", Handler: handlerVerifyProposal},
		{MethodName: "ExtendVote", Handler: handlerExtendVote},
		{MethodName: "VerifyExtension", Handler: handlerVerifyExtension},
		{MethodName: "AvailableSnapshots", Handler: handlerAvailableSnapshots},
		{MethodName: "Simulate", Handler: handlerSimulate},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ExportSnapshot",
			Handler:       handlerExportSnapshot,
			ServerStreams: true,
			ClientStreams: false,
		},
		{
			StreamName:    "ImportSnapshot",
			Handler:       handlerImportSnapshot,
			ServerStreams: false,
			ClientStreams: true,
		},
	},
	Metadata: "github.com/blockberries/bapi/v1/service.cram",
}
