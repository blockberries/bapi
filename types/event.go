package types

// EventAttribute is a single key-value tag within an event.
type EventAttribute struct {
	Key   string `cramberry:"1"`
	Value string `cramberry:"2"`
	Index bool   `cramberry:"3"` // Whether indexers should pick this up.
}

// Event is an application-emitted event.
type Event struct {
	Kind       string           `cramberry:"1"`
	Attributes []EventAttribute `cramberry:"2"`
}
