package types_test

import (
	"testing"
	"time"

	"github.com/blockberries/bapi/types"

	"github.com/blockberries/cramberry/pkg/cramberry"
)

// roundTrip marshals v, unmarshals into a new T, and returns it.
func roundTrip[T any](t *testing.T, v T) T {
	t.Helper()
	data, err := cramberry.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var out T
	if err := cramberry.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	return out
}

func TestTimestamp_RoundTrip(t *testing.T) {
	ts := types.TimeToTimestamp(time.Date(2024, 6, 15, 12, 30, 45, 123456789, time.UTC))
	got := roundTrip(t, ts)
	if got != ts {
		t.Fatalf("Timestamp round-trip failed: got %+v, want %+v", got, ts)
	}
	// Verify conversion back to time.Time.
	goTime := got.ToTime()
	if goTime.Year() != 2024 || goTime.Month() != 6 || goTime.Day() != 15 {
		t.Fatalf("Timestamp.ToTime date wrong: %v", goTime)
	}
	if goTime.Nanosecond() != 123456789 {
		t.Fatalf("Timestamp.ToTime nanos wrong: %d", goTime.Nanosecond())
	}
}

func TestDuration_RoundTrip(t *testing.T) {
	d := types.DurationFromGo(24 * time.Hour)
	got := roundTrip(t, d)
	if got != d {
		t.Fatalf("Duration round-trip failed: got %+v, want %+v", got, d)
	}
	if got.ToGo() != 24*time.Hour {
		t.Fatalf("Duration.ToGo wrong: %v", got.ToGo())
	}
}

func TestBlockID_RoundTrip(t *testing.T) {
	v := types.BlockID{Height: 42, Hash: types.Hash{1, 2, 3}}
	got := roundTrip(t, v)
	if got != v {
		t.Fatalf("BlockID round-trip failed: got %+v, want %+v", got, v)
	}
}

func TestEventAttribute_RoundTrip(t *testing.T) {
	v := types.EventAttribute{Key: "action", Value: "send", Index: true}
	got := roundTrip(t, v)
	if got != v {
		t.Fatalf("EventAttribute round-trip failed: got %+v, want %+v", got, v)
	}
}

func TestEvent_RoundTrip(t *testing.T) {
	v := types.Event{
		Kind: "transfer",
		Attributes: []types.EventAttribute{
			{Key: "from", Value: "alice", Index: true},
			{Key: "to", Value: "bob", Index: true},
			{Key: "amount", Value: "100", Index: false},
		},
	}
	got := roundTrip(t, v)
	if got.Kind != v.Kind || len(got.Attributes) != len(v.Attributes) {
		t.Fatalf("Event round-trip failed")
	}
	for i := range v.Attributes {
		if got.Attributes[i] != v.Attributes[i] {
			t.Fatalf("Event.Attributes[%d] mismatch", i)
		}
	}
}

func TestTxOutcome_RoundTrip(t *testing.T) {
	v := types.TxOutcome{
		Index:  3,
		Code:   0,
		Info:   "ok",
		Data:   []byte{0xDE, 0xAD},
		Events: []types.Event{{Kind: "exec", Attributes: nil}},
	}
	got := roundTrip(t, v)
	if got.Index != v.Index || got.Code != v.Code || got.Info != v.Info {
		t.Fatalf("TxOutcome round-trip failed: got %+v", got)
	}
}

func TestBlockOutcome_RoundTrip(t *testing.T) {
	v := types.BlockOutcome{
		TxOutcomes:  []types.TxOutcome{{Index: 0, Code: 0}},
		BlockEvents: []types.Event{{Kind: "reward"}},
		AppHash:     types.AppHash{0xAB},
		ValidatorUpdates: []types.ValidatorUpdate{
			{PubKey: types.PublicKey{Type: types.KeyTypeEd25519, Data: []byte{1}}, Power: 10},
		},
	}
	got := roundTrip(t, v)
	if got.AppHash != v.AppHash {
		t.Fatalf("BlockOutcome.AppHash mismatch")
	}
	if len(got.TxOutcomes) != 1 || len(got.ValidatorUpdates) != 1 {
		t.Fatalf("BlockOutcome slice lengths wrong")
	}
}

func TestFinalizedBlock_RoundTrip(t *testing.T) {
	v := types.FinalizedBlock{
		Height:        100,
		Time:          types.TimeToTimestamp(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		Proposer:      types.ValidatorAddress{0x01},
		Txs:           []types.Tx{[]byte("tx1"), []byte("tx2")},
		LastBlockHash: types.Hash{0xFF},
		VoteExtensions: []types.CommittedVoteExtension{
			{Validator: types.ValidatorAddress{0x02}, Extension: []byte("ext")},
		},
	}
	got := roundTrip(t, v)
	if got.Height != v.Height || got.Proposer != v.Proposer {
		t.Fatalf("FinalizedBlock round-trip failed")
	}
	if len(got.Txs) != 2 || len(got.VoteExtensions) != 1 {
		t.Fatalf("FinalizedBlock slices wrong")
	}
}

func TestCommitResult_RoundTrip(t *testing.T) {
	v := types.CommitResult{RetainHeight: 50}
	got := roundTrip(t, v)
	if got != v {
		t.Fatalf("CommitResult round-trip failed")
	}
}

func TestHandshakeRequest_RoundTrip(t *testing.T) {
	bid := types.BlockID{Height: 10, Hash: types.Hash{0x01}}
	v := types.HandshakeRequest{LastCommitted: &bid}
	got := roundTrip(t, v)
	if got.LastCommitted == nil || got.LastCommitted.Height != 10 {
		t.Fatalf("HandshakeRequest round-trip failed")
	}
}

func TestHandshakeResponse_RoundTrip(t *testing.T) {
	ah := types.AppHash{0xBE, 0xEF}
	v := types.HandshakeResponse{
		AppHash:      &ah,
		Capabilities: types.CapProposalControl | types.CapVoteExtensions,
	}
	got := roundTrip(t, v)
	if got.AppHash == nil || *got.AppHash != ah {
		t.Fatalf("HandshakeResponse.AppHash mismatch")
	}
	if !got.Capabilities.Has(types.CapProposalControl) {
		t.Fatalf("HandshakeResponse.Capabilities missing ProposalControl")
	}
}

func TestVoteContext_RoundTrip(t *testing.T) {
	v := types.VoteContext{Height: 5, Round: 2, BlockHash: types.Hash{0xAA}}
	got := roundTrip(t, v)
	if got != v {
		t.Fatalf("VoteContext round-trip failed")
	}
}

func TestReceivedExtension_RoundTrip(t *testing.T) {
	v := types.ReceivedExtension{
		Height: 5, Round: 2,
		Validator: types.ValidatorAddress{0x01},
		Extension: []byte("sig-data"),
	}
	got := roundTrip(t, v)
	if got.Height != v.Height || got.Validator != v.Validator {
		t.Fatalf("ReceivedExtension round-trip failed")
	}
}

func TestPublicKey_RoundTrip(t *testing.T) {
	v := types.PublicKey{Type: types.KeyTypeSecp256k1, Data: []byte{0x02, 0x03}}
	got := roundTrip(t, v)
	if got.Type != v.Type || len(got.Data) != len(v.Data) {
		t.Fatalf("PublicKey round-trip failed")
	}
}

func TestValidatorUpdate_RoundTrip(t *testing.T) {
	v := types.ValidatorUpdate{
		PubKey: types.PublicKey{Type: types.KeyTypeEd25519, Data: []byte{1, 2, 3}},
		Power:  1000,
	}
	got := roundTrip(t, v)
	if got.Power != v.Power {
		t.Fatalf("ValidatorUpdate round-trip failed")
	}
}

func TestProposalContext_RoundTrip(t *testing.T) {
	v := types.ProposalContext{
		Height:     10,
		Time:       types.TimeToTimestamp(time.Now()),
		Proposer:   types.ValidatorAddress{0x01},
		MempoolTxs: []types.Tx{[]byte("tx")},
		MaxTxBytes: 1024,
	}
	got := roundTrip(t, v)
	if got.Height != v.Height || got.MaxTxBytes != v.MaxTxBytes {
		t.Fatalf("ProposalContext round-trip failed")
	}
}

func TestProposalVerdict_RoundTrip(t *testing.T) {
	v := types.ProposalVerdict{Accept: false, RejectReason: "bad oracle tx"}
	got := roundTrip(t, v)
	if got.Accept != false || got.RejectReason != "bad oracle tx" {
		t.Fatalf("ProposalVerdict round-trip failed")
	}
}

func TestStateQuery_RoundTrip(t *testing.T) {
	h := uint64(42)
	v := types.StateQuery{Path: "/balance", Data: []byte("addr"), Height: &h, Prove: true}
	got := roundTrip(t, v)
	if string(got.Path) != "/balance" || got.Height == nil || *got.Height != 42 {
		t.Fatalf("StateQuery round-trip failed")
	}
}

func TestStateQueryResult_RoundTrip(t *testing.T) {
	v := types.StateQueryResult{
		Code:   0,
		Key:    []byte("k"),
		Value:  []byte("v"),
		Height: 10,
		Info:   "found",
	}
	got := roundTrip(t, v)
	if got.Code != 0 || got.Height != 10 {
		t.Fatalf("StateQueryResult round-trip failed")
	}
}

func TestGateVerdict_RoundTrip(t *testing.T) {
	v := types.GateVerdict{Code: 0, Info: "", Priority: 100, Sender: "alice"}
	got := roundTrip(t, v)
	if got.Priority != 100 || got.Sender != "alice" {
		t.Fatalf("GateVerdict round-trip failed")
	}
}

func TestGenesisDoc_RoundTrip(t *testing.T) {
	v := types.GenesisDoc{
		ChainID:       "test",
		GenesisTime:   types.TimeToTimestamp(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		InitialHeight: 1,
		ConsensusParams: types.ConsensusParams{
			MaxBlockBytes:  1048576,
			MaxEvidenceAge: types.DurationFromGo(24 * time.Hour),
			MaxTxBytes:     65536,
		},
		AppState: []byte(`{"init":true}`),
	}
	got := roundTrip(t, v)
	if got.ChainID != "test" || got.InitialHeight != 1 {
		t.Fatalf("GenesisDoc round-trip failed")
	}
}

func TestEvidence_RoundTrip(t *testing.T) {
	v := types.Evidence{
		Type:             types.EvidenceTypeDuplicateVote,
		Validator:        types.ValidatorAddress{0x01},
		Height:           50,
		Time:             types.TimeToTimestamp(time.Now()),
		TotalVotingPower: 1000,
	}
	got := roundTrip(t, v)
	if got.Type != v.Type || got.Height != 50 {
		t.Fatalf("Evidence round-trip failed")
	}
}

func TestConsensusParams_RoundTrip(t *testing.T) {
	v := types.ConsensusParams{
		MaxBlockBytes:  1048576,
		MaxEvidenceAge: types.DurationFromGo(time.Hour),
		MaxTxBytes:     65536,
	}
	got := roundTrip(t, v)
	if got.MaxBlockBytes != v.MaxBlockBytes || got.MaxTxBytes != v.MaxTxBytes {
		t.Fatalf("ConsensusParams round-trip failed")
	}
	if got.MaxEvidenceAge.ToGo() != time.Hour {
		t.Fatalf("ConsensusParams.MaxEvidenceAge round-trip failed")
	}
}

func TestSnapshotDescriptor_RoundTrip(t *testing.T) {
	v := types.SnapshotDescriptor{
		Height: 100, Format: 1, Chunks: 4,
		Hash:     types.Hash{0xAA},
		Metadata: []byte("gzip"),
	}
	got := roundTrip(t, v)
	if got.Height != 100 || got.Chunks != 4 {
		t.Fatalf("SnapshotDescriptor round-trip failed")
	}
}

func TestSnapshotChunk_RoundTrip(t *testing.T) {
	v := types.SnapshotChunk{Index: 2, Data: []byte("chunk-data")}
	got := roundTrip(t, v)
	if got.Index != 2 || string(got.Data) != "chunk-data" {
		t.Fatalf("SnapshotChunk round-trip failed")
	}
}

func TestImportResult_RoundTrip(t *testing.T) {
	ah := types.AppHash{0xBE}
	v := types.ImportResult{
		Status:  types.ImportOK,
		AppHash: &ah,
	}
	got := roundTrip(t, v)
	if got.Status != types.ImportOK || got.AppHash == nil || *got.AppHash != ah {
		t.Fatalf("ImportResult round-trip failed")
	}
}

func TestImportResult_RetryChunks_RoundTrip(t *testing.T) {
	v := types.ImportResult{
		Status:       types.ImportRetryChunks,
		RetryIndices: []uint32{0, 3, 7},
	}
	got := roundTrip(t, v)
	if got.Status != types.ImportRetryChunks || len(got.RetryIndices) != 3 {
		t.Fatalf("ImportResult retry round-trip failed")
	}
}

// TestDeterminism verifies that the same struct always produces
// the same bytes (cramberry's core guarantee).
func TestDeterminism(t *testing.T) {
	v := types.FinalizedBlock{
		Height:        42,
		Time:          types.Timestamp{Seconds: 1000, Nanos: 500},
		Proposer:      types.ValidatorAddress{0xAA},
		Txs:           []types.Tx{[]byte("a"), []byte("b")},
		LastBlockHash: types.Hash{0xFF},
	}
	data1, err := cramberry.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	data2, err := cramberry.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(data1) != len(data2) {
		t.Fatalf("non-deterministic: len %d vs %d", len(data1), len(data2))
	}
	for i := range data1 {
		if data1[i] != data2[i] {
			t.Fatalf("non-deterministic at byte %d: 0x%02x vs 0x%02x", i, data1[i], data2[i])
		}
	}
}
