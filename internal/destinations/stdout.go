package destinations

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jdwit/aws-lb-log-forwarder/internal/types"
)

// Stdout writes log entries to standard output.
type Stdout struct{}

// NewStdout creates a stdout destination.
func NewStdout() *Stdout {
	return &Stdout{}
}

// SendLogs writes each entry as JSON to stdout.
func (s *Stdout) SendLogs(ctx context.Context, entries <-chan types.LogEntry) {
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-entries:
			if !ok {
				return
			}

			data, err := json.Marshal(entry.Data)
			if err != nil {
				slog.Error("marshal failed", "error", err)
				continue
			}

			fmt.Printf("[%s] %s\n", entry.Timestamp.Format(time.RFC3339), data)
		}
	}
}
