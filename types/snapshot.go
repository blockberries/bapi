package types

// SnapshotDescriptor describes an available snapshot.
type SnapshotDescriptor struct {
	Height uint64 `cramberry:"1"`
	Format uint32 `cramberry:"2"`
	// Total number of chunks (for progress reporting).
	Chunks uint32 `cramberry:"3"`
	// Hash of the full snapshot (for integrity verification).
	Hash Hash `cramberry:"4"`
	// Arbitrary metadata (e.g., compression algo, version).
	Metadata []byte `cramberry:"5"`
}

// SnapshotChunk is a single piece of a snapshot.
type SnapshotChunk struct {
	Index uint32 `cramberry:"1"`
	Data  []byte `cramberry:"2"`
}

// ImportStatus describes the outcome of a snapshot import.
type ImportStatus uint8

const (
	// ImportOK means the snapshot was applied successfully.
	ImportOK ImportStatus = 1
	// ImportReject means the snapshot was rejected; try a different one.
	ImportReject ImportStatus = 2
	// ImportRetryChunks means some chunks were corrupt;
	// request these indices again.
	ImportRetryChunks ImportStatus = 3
)

// ImportResult is the outcome of importing a snapshot.
type ImportResult struct {
	Status ImportStatus `cramberry:"1"`
	// Set when Status is ImportOK.
	AppHash *AppHash `cramberry:"2"`
	// Set when Status is ImportReject.
	Reason string `cramberry:"3"`
	// Set when Status is ImportRetryChunks.
	RetryIndices []uint32 `cramberry:"4"`
}
