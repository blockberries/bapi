# BAPI

**Block Application Programming Interface** — a ground-up redesign of the consensus-application boundary for BFT consensus engines.

BAPI replaces ABCI 2.0 with a smaller, capability-oriented interface: 5 required methods cover the full lifecycle, 4 optional interfaces unlock advanced features, and the entire wire format uses deterministic binary serialization via [cramberry](https://github.com/blockberries/cramberry) (no protobuf).

## Quick Start

### Implement the Lifecycle Interface

Every BAPI application must implement `Lifecycle`:

```go
package myapp

import (
    "github.com/blockberries/bapi"
    "github.com/blockberries/bapi/types"
    "context"
)

type App struct{}

func (a *App) Handshake(ctx context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error) {
    // Initialize on genesis or report state on restart.
    if req.LastCommitted == nil {
        h := types.AppHash{} // your initial app hash
        return types.HandshakeResponse{AppHash: &h}, nil
    }
    // ... report persisted state
}

func (a *App) CheckTx(ctx context.Context, tx types.Tx, mctx types.MempoolContext) (types.GateVerdict, error) {
    // Validate transaction for mempool admission.
    return types.GateVerdict{Code: 0}, nil
}

func (a *App) ExecuteBlock(ctx context.Context, block types.FinalizedBlock) (types.BlockOutcome, error) {
    // Deterministically execute all transactions. Do NOT persist to disk.
    return types.BlockOutcome{AppHash: types.AppHash{0x01}}, nil
}

func (a *App) Commit(ctx context.Context) (types.CommitResult, error) {
    // Atomically persist state changes from the last ExecuteBlock.
    return types.CommitResult{}, nil
}

func (a *App) Query(ctx context.Context, req types.StateQuery) (types.StateQueryResult, error) {
    // Read application state.
    return types.StateQueryResult{}, nil
}
```

### Connect In-Process

```go
import "github.com/blockberries/bapi/local"

conn := local.NewConnection(&App{})
defer conn.Close()

resp, err := conn.Handshake(ctx, types.HandshakeRequest{
    Genesis: &types.GenesisDoc{ChainID: "my-chain"},
})
```

### Connect Over gRPC

```go
import bapigrpc "github.com/blockberries/bapi/grpc"

// Server
gs := grpc.NewServer()
bapigrpc.NewGRPCServer(&App{}).Register(gs)
gs.Serve(listener)

// Client
client, err := bapigrpc.Dial(ctx, "localhost:26658", grpc.WithInsecure())
defer client.Close()

resp, err := client.Handshake(ctx, types.HandshakeRequest{
    Genesis: &types.GenesisDoc{ChainID: "my-chain"},
})
```

## Interfaces

### Required: Lifecycle

| Method | Purpose | Concurrency |
|---|---|---|
| `Handshake` | Bootstrap on genesis or report state on restart | Called once at startup |
| `CheckTx` | Gate-check transactions for mempool admission | Concurrent |
| `ExecuteBlock` | Deterministically execute a finalized block | Sequential |
| `Commit` | Persist state changes to disk | Sequential, after ExecuteBlock |
| `Query` | Read application state | Concurrent |

### Optional Capabilities

Implement any combination of these interfaces and declare the corresponding capability bits in your `HandshakeResponse`:

| Interface | Bit | Purpose |
|---|---|---|
| `ProposalControl` | `CapProposalControl` | Control block contents and ordering |
| `VoteExtender` | `CapVoteExtensions` | Attach data to precommit votes |
| `StateSync` | `CapStateSync` | Snapshot-based fast sync |
| `Simulator` | `CapSimulation` | Dry-run transaction execution |

```go
func (a *App) Handshake(ctx context.Context, req types.HandshakeRequest) (types.HandshakeResponse, error) {
    return types.HandshakeResponse{
        AppHash:      &myHash,
        Capabilities: types.CapProposalControl | types.CapStateSync,
    }, nil
}
```

The engine validates at handshake that declared capabilities match implemented interfaces. Undeclared but implemented interfaces trigger a warning; declared but unimplemented interfaces cause an error.

## Package Overview

| Package | Description |
|---|---|
| `bapi` | Interface definitions (`Lifecycle`, `ProposalControl`, `VoteExtender`, `StateSync`, `Simulator`, `Connection`) |
| `types/` | 31 domain types with cramberry struct tags for deterministic serialization |
| `server/` | Engine-side wrapper enforcing the lifecycle state machine |
| `local/` | In-process adapter (zero serialization overhead) |
| `grpc/` | gRPC transport using cramberry codec (no protobuf) |
| `testing/` | `MockApp`, `Harness`, and `RunComplianceSuite` |
| `example/counter/` | Minimal app: transaction counter (Lifecycle only) |
| `example/dex/` | Full-featured app: DEX with all 4 optional capabilities |

## Examples

### Counter

A minimal application that counts transactions. Demonstrates `Lifecycle` with no optional capabilities.

```go
import "github.com/blockberries/bapi/example/counter"

app := counter.New()
conn := local.NewConnection(app)
```

See [`example/counter/app.go`](example/counter/app.go).

### DEX

A decentralized exchange demonstrating all optional capabilities: proposal control (oracle price injection), vote extensions (price feeds), state sync (JSON snapshots), and simulation (dry-run execution).

```go
import "github.com/blockberries/bapi/example/dex"

app := dex.New()
conn := local.NewConnection(app)
```

See [`example/dex/app.go`](example/dex/app.go).

## Testing

### Run All Tests

```bash
make test          # without race detector
make test-race     # with race detector
```

### Use the Test Harness

The `testing/` package provides tools for testing your application:

```go
import bapitest "github.com/blockberries/bapi/testing"

func TestMyApp(t *testing.T) {
    app := myapp.New()
    h := bapitest.NewHarness(t, app)

    // Genesis handshake with default parameters.
    h.GenesisDefault()

    // Execute a block with transactions.
    tx := types.Tx([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
    outcome := h.ExecuteAndCommit(bapitest.MakeBlock(1, tx))

    // Assert mempool behavior.
    h.MustAcceptTx(validTx)
    h.MustRejectTx(invalidTx)

    // Query state.
    result := h.Query("/my/path", nil)
}
```

### Run the Compliance Suite

Verify your application satisfies the BAPI contract:

```go
func TestCompliance(t *testing.T) {
    bapitest.RunComplianceSuite(t, func() bapi.Lifecycle {
        return myapp.New()
    })
}
```

The suite tests genesis handshake, execute/commit cycles, determinism, concurrent access, and transaction outcome correctness.

## Wire Format

BAPI uses [cramberry](https://github.com/blockberries/cramberry) for deterministic binary serialization. All domain types carry `cramberry:"N"` struct tags. The canonical field assignments are documented in:

- [`grpc/schema/bapi/v1/types.cram`](grpc/schema/bapi/v1/types.cram) — Type definitions
- [`grpc/schema/bapi/v1/service.cram`](grpc/schema/bapi/v1/service.cram) — Service definition

The gRPC transport registers a custom `CramberryCodec` that serializes domain types directly, eliminating the need for protobuf code generation or a type conversion layer.

## Building

```bash
make build   # go build ./...
make lint    # golangci-lint (or go vet as fallback)
make schema  # validate cramberry schemas (if CLI available)
make clean   # go clean ./...
```

Requires Go 1.25.6+.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed documentation of the internal design, including the lifecycle state machine, capability system, transport layers, and type system.
