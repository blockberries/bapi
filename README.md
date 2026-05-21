# BAPI — Block Application Programming Interface

The consensus↔application boundary for the Stealth stack. A ground-up
redesign of ABCI 2.0: smaller surface, capability-oriented, cramberry-encoded.

## What's different from ABCI

| ABCI 2.0                           | BAPI                                            |
|------------------------------------|--------------------------------------------------|
| `Info`, `InitChain`, `CheckTx`, `PrepareProposal`, `ProcessProposal`, `ExtendVote`, `VerifyVoteExtension`, `FinalizeBlock`, `Commit`, `Query` (10 methods, all required) | 5 required methods + 4 optional capability interfaces |
| `BeginBlock`, per-tx `DeliverTx`, `EndBlock`, then commit | Single `ExecuteBlock(FinalizedBlock) → BlockOutcome` returns per-tx outcomes + AppHash atomically |
| Protobuf wire format               | Cramberry wire format (deterministic, smaller)   |
| Capability negotiation by version  | Capability discovery via Go type assertion      |

## The required interface

```go
type Lifecycle interface {
    Handshake(ctx context.Context, req HandshakeRequest) (HandshakeResponse, error)
    CheckTx(ctx context.Context, tx Tx, mc MempoolContext) (GateVerdict, error)
    ExecuteBlock(ctx context.Context, block FinalizedBlock) (BlockOutcome, error)
    Commit(ctx context.Context) (CommitResult, error)
    Query(ctx context.Context, query StateQuery) (StateQueryResult, error)
}
```

`Handshake` covers both genesis and restart. Cold start: app initializes
from `req.Genesis`. Warm start: app validates `req.LastCommitID` matches
its own state, returns its `LastCommitID`.

## Optional capabilities

Discovered at handshake time via Go type assertion on the `Lifecycle`:

- **`ProposalControl`** (`BuildProposal` / `VerifyProposal`) — replaces ABCI
  PrepareProposal/ProcessProposal.
- **`VoteExtender`** (`ExtendVote` / `VerifyExtension`).
- **`StateSync`** (`AvailableSnapshots` / `ExportSnapshot` / `ImportSnapshot`)
  — channel-based, not chunk-by-chunk.
- **`Simulator`** (`Simulate`) — read-only execution for fee estimation, etc.

```go
type Application interface {  // convenience union
    Lifecycle
    ProposalControl
    VoteExtender
    StateSync
    Simulator
}
```

### Planned: `MempoolObserver`

Forward-looking capability landing as Phase 0.2 of the tokenomics SDK plan
(`PLAN.md` §7). **Not yet implemented** — listed here so app authors can
design against the contract.

```go
// MempoolObserver — opt-in capability. Apps that implement it receive
// per-validator participation signals fired from raspberry's consensus
// adapter. Used to drive tokenomics (e.g. batches_certified_v,
// leader_blocks_v counters per epoch).
type MempoolObserver interface {
    // OnBatchCertified fires when a looseberry batch's header is
    // vote-certified into a DAG round cert.
    OnBatchCertified(validator types.ValidatorAddress, batchHash []byte, txCount uint32, byteCount uint64)
    // OnBlockConstructed fires when leaderberry builds a block. Lazy
    // leaders proposing empty blocks are filtered upstream — this only
    // fires when len(includedBatchHashes) >= 1.
    OnBlockConstructed(leader types.ValidatorAddress, includedBatchHashes [][]byte)
}
```

Apps that don't implement it observe no behavior change. Apps that do can
drive a participation tracker and fee-routing destination.

### Note on `GateVerdict.Priority` / `GateVerdict.Sender`

These fields have existed on `GateVerdict` since the type was introduced.
The looseberry mempool now actually consumes them end-to-end: apps that
populate `Priority` (higher = first) and `Sender` (same-sender sequencing
key) get priority-fee mempool ordering for free. See `example/dex/app.go`
for the canonical pattern.

## Transports

- **`local/`** — in-process `Connection`. Zero serialization. Used by
  raspberry to call its own application.
- **`grpc/`** — gRPC server and client with a custom `CramberryCodec`
  replacing protobuf. Schema in `grpc/schema/bapi/v1/`.
- **`server/`** — wraps a `Lifecycle` with a `LifecycleGuard` state machine
  that fail-fasts on protocol violations (e.g. `ExecuteBlock` before
  `Handshake`).

## Usage

```go
import (
    "github.com/blockberries/bapi"
    bapilocal "github.com/blockberries/bapi/local"
)

type myApp struct{ /* ... */ }
func (a *myApp) Handshake(ctx, req) (HandshakeResponse, error) { /* ... */ }
// ... 4 more methods ...

conn := bapilocal.NewConnection(&myApp{})
_, err := conn.Handshake(ctx, bapi.HandshakeRequest{...})
```

## Layout

```
bapi/
├── bapi.go              Lifecycle, ProposalControl, VoteExtender, StateSync, Simulator interfaces
├── errors.go            Sentinel errors
├── types/               Concrete types (FinalizedBlock, BlockOutcome, GenesisDoc, ...)
├── server/              LifecycleGuard, capability discovery
├── local/               In-process Connection
├── grpc/                gRPC server, client, codec, service
│   └── schema/bapi/v1/  .cram schemas + generated code
├── testing/             MockApp, Harness, RunComplianceSuite
└── example/
    ├── counter/         Minimal Lifecycle-only example
    └── dex/             All-capabilities example
```

## Development

See [`CLAUDE.md`](./CLAUDE.md) for development guidelines.
[`ARCHITECTURE.md`](./ARCHITECTURE.md) for design details.

## License

Apache-2.0.
