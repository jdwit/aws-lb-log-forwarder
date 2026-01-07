package outputs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jdwit/aws-lb-log-forwarder/internal/types"
)

// Output receives log entries and sends them to an output.
type Output interface {
	SendLogs(ctx context.Context, entries <-chan types.LogEntry)
}

// New creates outputs from a comma-separated configuration string.
func New(config string, sess *session.Session) ([]Output, error) {
	var result []Output

	for _, name := range strings.Split(config, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		var o Output
		var err error

		switch name {
		case "cloudwatch":
			o, err = NewCloudWatch(sess)
		case "firehose":
			o, err = NewFirehose(sess)
		case "splunk":
			o, err = NewSplunk()
		case "elasticsearch", "opensearch":
			o, err = NewElasticsearch()
		case "stdout":
			o = NewStdout()
		default:
			slog.Warn("unknown output", "name", name)
			continue
		}

		if err != nil {
			slog.Warn("output init failed", "name", name, "error", err)
			continue
		}

		result = append(result, o)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid outputs configured")
	}

	return result, nil
}
