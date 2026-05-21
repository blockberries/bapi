# BAPI — Architecture

## Design principles

1. **Smaller required surface than ABCI 2.0**: 5 methods, not 10.
2. **Capability-oriented**: optional interfaces unlock advanced features
   without bloating the core. Discovery via Go type assertion.
3. **Cramberry wire format**: deterministic, smaller, no protobuf
   dependency.
4. **Whole-block execution**: `ExecuteBlock(FinalizedBlock) → BlockOutcome`
   replaces BeginBlock + per-tx DeliverTx + EndBlock. Atomic from the
   engine's perspective.
5. **Both restart and genesis through Handshake**: a single entry point
   covers both cold start and warm start, with the app deciding from the
   request shape which case applies.

## The `Lifecycle` contract

```go
Handshake(ctx, HandshakeRequest)        → HandshakeResponse
CheckTx(ctx, Tx, MempoolContext)        → GateVerdict
ExecuteBlock(ctx, FinalizedBlock)       → BlockOutcome
Commit(ctx)                             → CommitResult
Query(ctx, StateQuery)                  → StateQueryResult
```

State machine, enforced by `server/LifecycleGuard`:

```
[init] ──Handshake──▶ [ready] ──CheckTx──▶ [ready]
                          │
                     ExecuteBlock
                          │
                          ▼
                    [executed] ──Commit──▶ [ready]
                          │
                       Query (allowed in [ready] only)
```

`Query` and `CheckTx` are concurrent-safe; `ExecuteBlock` and `Commit` are
strictly sequential.

## Capability interfaces

```go
ProposalControl  → BuildProposal, VerifyProposal
VoteExtender     → ExtendVote, VerifyExtension
StateSync        → AvailableSnapshots, ExportSnapshot, ImportSnapshot
Simulator        → Simulate
```

Discovered:

```go
func discoverCapabilities(app Lifecycle) Capabilities {
    var caps Capabilities
    if _, ok := app.(ProposalControl); ok { caps.Set(CapProposalControl) }
    if _, ok := app.(VoteExtender);    ok { caps.Set(CapVoteExtensions) }
    if _, ok := app.(StateSync);       ok { caps.Set(CapStateSync) }
    if _, ok := app.(Simulator);       ok { caps.Set(CapSimulation) }
    return caps
}
```

The engine then uses the matching `Connection.As<X>()` accessor to obtain a
typed handle; if the capability is absent, `As<X>()` returns `nil`.

### Planned capability: `MempoolObserver`

Status: **planned**, landing as Phase 0.2 of the tokenomics SDK plan
(`PLAN.md` §7). No bapi code changes yet — this section documents the
contract being designed against so downstream work can proceed in
parallel.

```go
MempoolObserver  → OnBatchCertified, OnBlockConstructed
```

Signature:

```go
type MempoolObserver interface {
    OnBatchCertified(validator types.ValidatorAddress, batchHash []byte, txCount uint32, byteCount uint64)
    OnBlockConstructed(leader types.ValidatorAddress, includedBatchHashes [][]byte)
}
```

Semantics:

- `OnBatchCertified` fires from raspberry's consensus adapter when a
  looseberry batch reaches cert-quorum — i.e., its header was
  vote-certified into a DAG round cert. Tokenomics modules use this to
  count `batches_certified_v` per validator per epoch.
- `OnBlockConstructed` fires when leaderberry builds a block. Tokenomics
  modules use this to count `leader_blocks_v` per validator per epoch.
  Only fires when `len(includedBatchHashes) >= 1`; lazy leaders proposing
  empty blocks get no credit.

Discovery will follow the same pattern as the existing capabilities (Go
type assertion at handshake time, capped by the same declare/implement
consistency check in `server/server.go`). When integrated, the discovery
block above grows by one branch:

```go
if _, ok := app.(MempoolObserver); ok { caps.Set(CapMempoolObserver) }
```

The capability is opt-in. Apps that don't implement it observe no
behavior change. Apps that do can drive a participation tracker and a
fee-routing destination from the same callbacks.

## Mempool ordering: `GateVerdict.Priority` and `GateVerdict.Sender`

`GateVerdict` carries two fields beyond the accept/reject signal:

```go
type GateVerdict struct {
    Code     uint32
    Info     string
    Priority int64   // Higher = first.
    Sender   string  // Same-sender sequencing / replacement key.
}
```

These fields are now consumed end-to-end by the looseberry mempool: apps
that populate them get priority-fee mempool ordering and same-sender
sequencing without any extra wiring. The dex example at
`example/dex/app.go` is the canonical pattern — it sets `Priority` from
the transaction's fee tier and `Sender` from the address prefix.

Apps that leave both fields at their zero values fall through to FIFO
ordering, which remains the default for the counter example.

## Wire format

All BAPI types are defined in `types/` as plain Go structs with
`cramberry:` tags. Generated schemas live in `grpc/schema/bapi/v1/`.

Cramberry is used for both:
- gRPC payloads (`grpc/codec.go::CramberryCodec`).
- In-process method calls (`local/connection.go` — actually skips
  serialization for performance; the cramberry-encoded shape is
  authoritative for any transport).

## Transports

### `local/`

Zero-serialization in-process transport. The `Connection` simply holds a
`Lifecycle` and forwards calls. Used by raspberry — its application lives
in the same process as the engine, so no serialization is required.

### `grpc/`

Real gRPC transport for out-of-process apps. Custom `CramberryCodec`
replaces the default protobuf codec. Service definitions are generated
from `grpc/schema/bapi/v1/service.cram`.

Both client and server are stateless beyond the connection — protocol
state is enforced by `LifecycleGuard` on the server side.

## Lifecycle guard

`server/state_machine.go` enforces the protocol state machine. On a
violation (e.g. `Commit` before `ExecuteBlock`), it `panic()`s. The
expectation is that the consensus engine is correctly written; a panic
here is a bug to fix in the engine, not in the app.

## Compliance suite

`testing/compliance.go` exposes `RunComplianceSuite(t, factory)` which
exercises every method of every interface with both happy-path and
error cases:

- Genesis handshake correctness
- Restart handshake idempotency
- CheckTx → ExecuteBlock → Commit cycle
- Query before/after Commit
- Capability discovery
- Concurrent CheckTx + Query

App authors should run this against their implementation.

## Genesis & state sync

`HandshakeRequest.Genesis` carries the application state at chain start.
For state sync, `StateSync.ImportSnapshot` accepts a channel of chunks and
streams them in; the app decides how to materialize state.

## Layout

```
bapi/
├── bapi.go                Lifecycle + capability interfaces
├── errors.go              Sentinel errors
├── types/                 FinalizedBlock, BlockOutcome, Tx, ...
│                          + cramberry tags + generated companion code
├── server/
│   ├── server.go          Wraps Lifecycle with LifecycleGuard, discovers caps
│   └── state_machine.go   Protocol state enforcement
├── local/
│   └── local.go           In-process Connection
├── grpc/
│   ├── codec.go           CramberryCodec replacing protobuf
│   ├── client.go          gRPC client
│   ├── server.go          gRPC server
│   ├── service.go         Service routing
│   ├── wire.go            Wire helpers
│   └── schema/bapi/v1/    .cram schemas + generated code
├── testing/
│   ├── mockapp.go
│   ├── harness.go
│   └── compliance.go
└── example/
    ├── counter/           Minimal Lifecycle-only example
    └── dex/               All-capabilities example
```
