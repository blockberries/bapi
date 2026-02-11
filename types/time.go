package types

import "time"

// Timestamp is a wire-safe representation of a point in time.
// Uses seconds since Unix epoch plus a nanosecond offset,
// ensuring deterministic serialization across languages.
type Timestamp struct {
	Seconds int64 `cramberry:"1"`
	Nanos   int32 `cramberry:"2"`
}

// TimeToTimestamp converts a time.Time to a Timestamp.
func TimeToTimestamp(t time.Time) Timestamp {
	return Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

// ToTime converts a Timestamp to a time.Time (UTC).
func (ts Timestamp) ToTime() time.Time {
	return time.Unix(ts.Seconds, int64(ts.Nanos)).UTC()
}

// Duration is a wire-safe representation of a time duration.
// Stored as nanoseconds for maximum precision.
type Duration struct {
	Nanos int64 `cramberry:"1"`
}

// DurationFromGo converts a time.Duration to a Duration.
func DurationFromGo(d time.Duration) Duration {
	return Duration{Nanos: d.Nanoseconds()}
}

// ToGo converts a Duration to a time.Duration.
func (d Duration) ToGo() time.Duration {
	return time.Duration(d.Nanos)
}
