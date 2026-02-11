package types

// HandshakeRequest is sent by the engine on every startup.
type HandshakeRequest struct {
	// The last block the ENGINE committed. Nil = genesis (fresh chain).
	LastCommitted *BlockID `cramberry:"1"`
	// Raw genesis document. Only set when LastCommitted is nil.
	Genesis *GenesisDoc `cramberry:"2"`
}

// HandshakeResponse is the application's reply, reporting its
// state and capabilities.
type HandshakeResponse struct {
	// The last block the APP committed. Nil = app has no state.
	LastBlock *BlockID `cramberry:"1"`
	// App hash at that height (for consistency check with engine).
	AppHash *AppHash `cramberry:"2"`
	// Capabilities this app supports. Drives engine behavior.
	Capabilities Capabilities `cramberry:"3"`
}
