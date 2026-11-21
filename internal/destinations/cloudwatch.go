package destinations

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/jdwit/aws-lb-log-forwarder/internal/types"
)

const (
	maxBatchSize  = 1_048_576 // 1MB
	maxBatchCount = 10_000
	flushInterval = 5 * time.Second
)

// CloudWatchAPI defines the CloudWatch Logs operations used.
type CloudWatchAPI interface {
	PutLogEvents(context.Context, *cloudwatchlogs.PutLogEventsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
	CreateLogGroup(context.Context, *cloudwatchlogs.CreateLogGroupInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	CreateLogStream(context.Context, *cloudwatchlogs.CreateLogStreamInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	DescribeLogStreams(context.Context, *cloudwatchlogs.DescribeLogStreamsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
}

// CloudWatch sends log entries to CloudWatch Logs.
type CloudWatch struct {
	client    CloudWatchAPI
	logGroup  string
	logStream string
}

// NewCloudWatch creates a CloudWatch destination from environment configuration.
func NewCloudWatch(cfg aws.Config) (*CloudWatch, error) {
	group, err := requiredEnv("CLOUDWATCH_LOG_GROUP")
	if err != nil {
		return nil, err
	}

	stream, err := requiredEnv("CLOUDWATCH_LOG_STREAM")
	if err != nil {
		return nil, err
	}

	client := cloudwatchlogs.NewFromConfig(cfg)
	ctx := context.Background()

	if err := ensureLogGroup(ctx, client, group); err != nil {
		return nil, fmt.Errorf("ensure log group: %w", err)
	}
	if err := ensureLogStream(ctx, client, group, stream); err != nil {
		return nil, fmt.Errorf("ensure log stream: %w", err)
	}

	return &CloudWatch{
		client:    client,
		logGroup:  group,
		logStream: stream,
	}, nil
}

// SendLogs receives entries and batches them to CloudWatch.
func (c *CloudWatch) SendLogs(ctx context.Context, entries <-chan types.LogEntry) {
	var batch []cwtypes.InputLogEvent
	var batchSize int

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		c.send(batch)
		batch = nil
		batchSize = 0
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return

		case entry, ok := <-entries:
			if !ok {
				flush()
				return
			}

			data, err := json.Marshal(entry.Data)
			if err != nil {
				slog.Error("marshal failed", "error", err)
				continue
			}

			event := cwtypes.InputLogEvent{
				Message:   aws.String(string(data)),
				Timestamp: aws.Int64(entry.Timestamp.UnixMilli()),
			}

			eventSize := len(data) + 26 // CloudWatch overhead per event

			if len(batch) > 0 && (batchSize+eventSize > maxBatchSize || len(batch) >= maxBatchCount) {
				flush()
			}

			batch = append(batch, event)
			batchSize += eventSize

		case <-ticker.C:
			flush()
		}
	}
}

func (c *CloudWatch) send(events []cwtypes.InputLogEvent) {
	sort.Slice(events, func(i, j int) bool {
		return *events[i].Timestamp < *events[j].Timestamp
	})

	// Background context: the final flush on ctx.Done() must still be sent.
	_, err := c.client.PutLogEvents(context.Background(), &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     events,
		LogGroupName:  aws.String(c.logGroup),
		LogStreamName: aws.String(c.logStream),
	})
	if err != nil {
		slog.Error("put events failed", "error", err)
	}
}

func ensureLogGroup(ctx context.Context, client CloudWatchAPI, name string) error {
	resp, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(name),
	})
	if err != nil {
		return err
	}

	for _, g := range resp.LogGroups {
		if *g.LogGroupName == name {
			return nil
		}
	}

	slog.Info("creating log group", "name", name)
	_, err = client.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(name),
	})
	return err
}

func ensureLogStream(ctx context.Context, client CloudWatchAPI, group, stream string) error {
	resp, err := client.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(group),
		LogStreamNamePrefix: aws.String(stream),
	})
	if err != nil {
		return err
	}

	for _, s := range resp.LogStreams {
		if *s.LogStreamName == stream {
			return nil
		}
	}

	slog.Info("creating log stream", "group", group, "stream", stream)
	_, err = client.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(group),
		LogStreamName: aws.String(stream),
	})
	return err
}
