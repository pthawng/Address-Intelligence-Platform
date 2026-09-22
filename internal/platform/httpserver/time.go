package httpserver

import "time"

// Timestamp normalizes DTO timestamps to UTC. time.Time's JSON encoder emits
// RFC 3339 (ISO 8601), preserving available subsecond precision.
func Timestamp(value time.Time) time.Time { return value.UTC() }
