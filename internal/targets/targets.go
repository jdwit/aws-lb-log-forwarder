package targets

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jdwit/alb-log-pipe/internal/types"
)

// Target receives log entries and sends them to a destination.
type Target interface {
	SendLogs(ctx context.Context, entries <-chan types.LogEntry)
}

// New creates targets from a comma-separated configuration string.
func New(config string, sess *session.Session) ([]Target, error) {
	var result []Target

	for _, name := range strings.Split(config, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		var t Target
		var err error

		switch name {
		case "cloudwatch":
			t, err = NewCloudWatch(sess)
		case "stdout":
			t = NewStdout()
		default:
			slog.Warn("unknown target", "name", name)
			continue
		}

		if err != nil {
			slog.Warn("target init failed", "name", name, "error", err)
			continue
		}

		result = append(result, t)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid targets configured")
	}

	return result, nil
}
