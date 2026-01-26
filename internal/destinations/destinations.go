package destinations

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jdwit/aws-lb-log-forwarder/internal/types"
)

// Destination receives log entries and sends them to a destination.
type Destination interface {
	SendLogs(ctx context.Context, entries <-chan types.LogEntry)
}

// New creates destinations from a comma-separated configuration string.
func New(config string, sess *session.Session) ([]Destination, error) {
	var result []Destination

	for _, name := range strings.Split(config, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		var d Destination
		var err error

		switch name {
		case "cloudwatch":
			d, err = NewCloudWatch(sess)
		case "splunk":
			d, err = NewSplunk()
		case "opensearch":
			d, err = NewOpenSearch()
		case "stdout":
			d = NewStdout()
		default:
			slog.Warn("unknown destination", "name", name)
			continue
		}

		if err != nil {
			slog.Warn("destination init failed", "name", name, "error", err)
			continue
		}

		result = append(result, d)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid destinations configured")
	}

	return result, nil
}
