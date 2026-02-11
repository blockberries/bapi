package types

// GenesisDoc is the raw genesis document for chain initialization.
type GenesisDoc struct {
	ChainID         string            `cramberry:"1"`
	GenesisTime     Timestamp         `cramberry:"2"`
	InitialHeight   uint64            `cramberry:"3"`
	ConsensusParams ConsensusParams   `cramberry:"4"`
	Validators      []ValidatorUpdate `cramberry:"5"`
	// Application-specific genesis state (typically JSON).
	AppState []byte `cramberry:"6"`
}
