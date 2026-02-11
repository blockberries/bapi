# BAPI Architecture

This document describes the internal architecture of the BAPI (Block Application Programming Interface) codebase. BAPI is a ground-up redesign of the consensus-application boundary, replacing ABCI 2.0 with a smaller, capability-oriented interface.

## Design Principles

1. **Minimal surface area.** Five required methods cover the entire lifecycle; everything else is optional.
2. **Capability discovery, not negotiation.** The application declares what it supports once at handshake; the engine adapts.
3. **Transport-agnostic.** The same application binary can be driven in-process (zero-copy) or over gRPC with no code changes.
4. **Deterministic serialization.** All wire types use [cramberry](https://github.com/blockberries/cramberry) struct tags for cross-language binary encoding. No protobuf.
5. **State machine enforcement.** The engine-side server enforces correct call ordering via a lifecycle guard, catching bugs at the boundary rather than inside the application.

## Package Map

```
bapi/
├── bapi.go              # Interface definitions (Lifecycle, optional capabilities, Connection)
├── errors.go            # HaltError for irrecoverable chain halts
├── types/               # All domain types (31 structs, cramberry-tagged)
│   ├── types.go         #   Hash, AppHash, Tx, QueryPath, BlockID
│   ├── time.go          #   Timestamp, Duration (wire-safe time representations)
│   ├── block.go         #   TxOutcome, BlockOutcome, FinalizedBlock, CommitResult
│   ├── event.go         #   EventAttribute, Event
│   ├── handshake.go     #   HandshakeRequest, HandshakeResponse
│   ├── mempool.go       #   MempoolContext, GateVerdict
│   ├── proposal.go      #   ProposalContext, BuiltProposal, ReceivedProposal, ProposalVerdict
│   ├── vote.go          #   VoteContext, ReceivedExtension, ExtensionVerdict, CommittedVoteExtension
│   ├── query.go         #   StateQuery, StateQueryResult, MerkleProof, ProofOp
│   ├── validator.go     #   PublicKey, ValidatorUpdate, ValidatorAddress, KeyType
│   ├── genesis.go       #   GenesisDoc
│   ├── evidence.go      #   EvidenceType, Evidence
│   ├── params.go        #   ConsensusParams
│   ├── snapshot.go      #   SnapshotDescriptor, SnapshotChunk, ImportStatus, ImportResult
│   └── capability.go    #   Capabilities bitfield and constants
├── server/              # Engine-side wrapper
│   ├── server.go        #   Server: lifecycle enforcement + capability routing
│   └── state_machine.go #   LifecycleGuard: atomic state machine
├── local/               # In-process adapter (zero-copy)
│   └── local.go         #   Connection wrapping server.Server
├── grpc/                # gRPC transport (cramberry codec)
│   ├── codec.go         #   CramberryCodec (grpc/encoding.Codec)
│   ├── wire.go          #   Transport-specific wrapper types
│   ├── service.go       #   Manual grpc.ServiceDesc (no codegen)
│   ├── server.go        #   GRPCServer: domain types over gRPC
│   ├── client.go        #   Client: bapi.Connection over gRPC
│   └── schema/          #   Cross-language wire format documentation
│       └── bapi/v1/
│           ├── types.cram
│           └── service.cram
├── testing/             # Test infrastructure
│   ├── mock.go          #   MockApp with configurable handlers
│   ├── harness.go       #   Harness for lifecycle testing
│   └── compliance.go    #   RunComplianceSuite
└── example/             # Reference applications
    ├── counter/         #   Minimal: Lifecycle only
    │   └── app.go
    └── dex/             #   Full-featured: all 4 optional capabilities
        └── app.go
```

## Interface Hierarchy

```
┌─────────────────────────────────────────────┐
│                 Lifecycle                    │  Required for every app
│  Handshake  CheckTx  ExecuteBlock           │
│  Commit  Query                              │
└─────────────────────────────────────────────┘
        ▲
        │ embeds
┌───────┴─────────────────────────────────────┐
│               Connection                    │  Transport abstraction
│  + Capabilities()                           │
│  + AsProposalControl() → ProposalControl    │
│  + AsVoteExtender()    → VoteExtender       │
│  + AsStateSync()       → StateSync          │
│  + AsSimulator()       → Simulator          │
│  + Close()                                  │
└─────────────────────────────────────────────┘
        ▲                         ▲
        │                         │
  local.Connection          grpc.Client
  (in-process)              (remote)
```

### Optional Capability Interfaces

| Interface | Capability Bit | Methods | Replaces (ABCI) |
|---|---|---|---|
| `ProposalControl` | `CapProposalControl` (0x01) | `BuildProposal`, `VerifyProposal` | PrepareProposal, ProcessProposal |
| `VoteExtender` | `CapVoteExtensions` (0x02) | `ExtendVote`, `VerifyExtension` | ExtendVote, VerifyVoteExtension |
| `StateSync` | `CapStateSync` (0x04) | `AvailableSnapshots`, `ExportSnapshot`, `ImportSnapshot` | ListSnapshots, LoadSnapshotChunk, OfferSnapshot, ApplySnapshotChunk |
| `Simulator` | `CapSimulation` (0x08) | `Simulate` | CheckTx (dry-run mode) |

Capability discovery works via Go type assertions for in-process connections and via the `Capabilities` bitfield over gRPC. The `server.Server` validates at handshake time that declared capabilities match implemented interfaces.

## Lifecycle State Machine

The `LifecycleGuard` in `server/state_machine.go` enforces this state machine using atomic operations and a sequential mutex:

```
                ┌──────────────────────────────────────┐
                │                                      │
                ▼                                      │
  ┌───────┐  Handshake  ┌───────┐  ExecuteBlock  ┌──────────┐
  │ Init  │ ──────────► │ Ready │ ──────────────► │Executing │
  └───────┘              └───────┘                └──────────┘
       ▲                     ▲                         │
       │                     │                         │ success
  fail │                     │ Commit               ┌──▼──────┐
       │                ┌────┴──────┐               │ Executed │
       └────────────────│Committing │◄──────────────┘         │
                        └───────────┘                └─────────┘
```

**Concurrency rules:**
- `CheckTx`, `Query`, and `Simulate` may run concurrently in any state after Handshake completes.
- `ExecuteBlock` and `Commit` are sequential and mutually exclusive.
- Calling a method in the wrong state panics (programming error in the engine, not a runtime condition).
- Failed `ExecuteBlock` rolls back to Ready; failed `Handshake` rolls back to Init.

## Transport Layers

### In-Process (`local/`)

`local.Connection` wraps `server.Server` directly. Zero serialization overhead. The engine and application share the same address space; domain types are passed by value.

```go
conn := local.NewConnection(myApp)
resp, _ := conn.Handshake(ctx, req)
```

### gRPC (`grpc/`)

The gRPC transport uses four components:

1. **CramberryCodec** (`codec.go`) — Registered via `encoding.RegisterCodec` at init time. Serializes domain types directly using `cramberry.Marshal`/`Unmarshal`. No protobuf, no code generation.

2. **Wire Types** (`wire.go`) — Nine wrapper structs for RPC methods whose Go interface signatures don't map 1:1 to a single request/response struct. These exist only at the serialization boundary (e.g., `CheckTxRequest` bundles `Tx` + `MempoolContext`).

3. **Service Descriptor** (`service.go`) — A hand-written `grpc.ServiceDesc` with 11 unary handlers and 2 streaming handlers. No `.proto` files or generated code. The `BAPIServiceServer` interface defines the server contract.

4. **Server/Client** (`server.go`, `client.go`) — The server wraps `server.Server` and implements `BAPIServiceServer`. The client implements `bapi.Connection` using `cc.Invoke` for unary RPCs and `cc.NewStream` for streaming. Capability wrappers (e.g., `clientProposalControl`) are thin types that forward calls to the underlying gRPC connection.

```go
// Server side
gs := grpc.NewServer()
bapigrpc.NewGRPCServer(myApp).Register(gs)

// Client side
client, _ := bapigrpc.Dial(ctx, "localhost:26658", grpc.WithInsecure())
resp, _ := client.Handshake(ctx, req)
```

**Streaming RPCs:**
- `ExportSnapshot` — Server-streaming. The server reads chunks from the app's channel and sends them to the client.
- `ImportSnapshot` — Client-streaming. The first message carries the `SnapshotDescriptor`; subsequent messages carry `SnapshotChunk`s. The server response contains the `ImportResult`.

## Type System

All 31 domain types live in `types/` with `cramberry:"N"` struct tags for deterministic binary serialization. Field numbering starts at 1 within each struct and must never be reused.

### Primitives

| Type | Underlying | Size |
|---|---|---|
| `Hash` | `[32]byte` | Fixed 32B |
| `AppHash` | `[32]byte` | Fixed 32B |
| `ValidatorAddress` | `[20]byte` | Fixed 20B |
| `Tx` | `[]byte` | Variable |
| `QueryPath` | `string` | Variable |
| `Capabilities` | `uint8` | 1B bitfield |
| `MempoolContext` | `uint8` | 1B enum |
| `EvidenceType` | `uint8` | 1B enum |
| `ExtensionVerdict` | `uint8` | 1B enum |
| `ImportStatus` | `uint8` | 1B enum |
| `KeyType` | `uint8` | 1B enum |

These type aliases need no cramberry tags — they serialize as their underlying types.

### Time Types

`time.Time` and `time.Duration` are not used in any wire type. Instead, `Timestamp` (seconds + nanos) and `Duration` (nanos) provide deterministic cross-language representations. Conversion helpers bridge to Go's standard library:

```go
ts := types.TimeToTimestamp(time.Now())
t  := ts.ToTime()

d  := types.DurationFromGo(5 * time.Second)
gd := d.ToGo()
```

### Schema Files

The canonical wire format is documented in `grpc/schema/bapi/v1/`:
- `types.cram` — Field assignments for all domain types.
- `service.cram` — Service definition and wire-only types.

These are human-readable and serve as the cross-language source of truth.

## Error Handling

**`HaltError`** — A special error type returned from `ExecuteBlock` when the application detects an irrecoverable inconsistency (e.g., state root mismatch). The engine must stop consensus immediately. Created via `NewHaltError(height, reason)` and detected via `IsHalt(err)`, which supports `errors.As` unwrapping.

All other errors from BAPI methods are treated as transient. Application-level error codes (e.g., invalid transaction) are communicated via result codes in `TxOutcome.Code` and `GateVerdict.Code`, not via Go errors.

## Testing Infrastructure

### MockApp (`testing/mock.go`)

A fully configurable mock implementing all BAPI interfaces. Every method is overridable via function fields (e.g., `ExecuteBlockFn`). Unconfigured methods return sensible defaults. Atomic call counters track invocations for assertions.

### Harness (`testing/harness.go`)

Wraps `server.Server` with convenience methods for application testing:
- `GenesisDefault()` — Handshake with a standard genesis doc.
- `ExecuteAndCommit(block)` — Execute + commit in one call.
- `MustAcceptTx(tx)` / `MustRejectTx(tx)` — Assert mempool admission.
- `MakeBlock(height, txs...)` / `MakeEmptyBlock(height)` — Factory functions.

### Compliance Suite (`testing/compliance.go`)

`RunComplianceSuite(t, factory)` runs a standard battery of tests against any `Lifecycle` implementation:
- Genesis handshake returns valid response
- Execute/commit cycle produces non-zero AppHash
- Determinism: same blocks on two instances produce identical AppHash
- Concurrent `CheckTx` and `Query` after handshake
- Transaction outcome indices match block position

## Example Applications

### Counter (`example/counter/`)

Minimal application demonstrating `Lifecycle` only. Counts transactions (8-byte big-endian uint64 increment values). No optional capabilities. AppHash is SHA-256 of the counter value.

### DEX (`example/dex/`)

Full-featured decentralized exchange implementing all four optional capabilities:

- **ProposalControl** — Aggregates oracle prices from vote extensions and injects an `oracle_update` transaction at the head of each block. Verifies proposal structure.
- **VoteExtensions** — Each validator publishes its current price feeds as a JSON vote extension. Extensions are verified for structural validity.
- **StateSync** — Serializes the full state as JSON, chunks it at 64 KiB boundaries, and verifies integrity via SHA-256 hash on import.
- **Simulation** — Dry-runs transactions against a cloned state without side effects.

Transaction types: `deposit` (0x01), `withdraw` (0x02), `limit_order` (0x03), `oracle_update` (0x04).

## Dependencies

| Module | Purpose |
|---|---|
| `github.com/blockberries/cramberry` v1.6.0 | Deterministic binary serialization |
| `google.golang.org/grpc` v1.72.0 | gRPC transport |

No other direct dependencies. Indirect dependencies come from gRPC's own requirements.
