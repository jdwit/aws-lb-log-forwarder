package types

import "time"

// LogEntry represents a parsed ALB log entry with its timestamp.
type LogEntry struct {
	Data      map[string]string
	Timestamp time.Time
}
