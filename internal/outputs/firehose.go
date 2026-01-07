package outputs

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/jdwit/alb-log-forwarder/internal/types"
)

const (
	// Firehose limits: 500 records or 4MB per batch
	firehoseMaxRecords   = 500
	firehoseMaxBatchSize = 4_000_000
)

// FirehoseAPI defines the Firehose operations used.
type FirehoseAPI interface {
	PutRecordBatch(*firehose.PutRecordBatchInput) (*firehose.PutRecordBatchOutput, error)
}

// Firehose sends log entries to Kinesis Data Firehose.
type Firehose struct {
	client     FirehoseAPI
	streamName string
}

// NewFirehose creates a Firehose output from environment configuration.
func NewFirehose(sess *session.Session) (*Firehose, error) {
	stream, err := requiredEnv("FIREHOSE_STREAM_NAME")
	if err != nil {
		return nil, err
	}

	return &Firehose{
		client:     firehose.New(sess),
		streamName: stream,
	}, nil
}

// SendLogs receives entries and batches them to Firehose.
func (f *Firehose) SendLogs(ctx context.Context, entries <-chan types.LogEntry) {
	var batch []*firehose.Record
	var batchSize int

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		f.send(batch)
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

			// Add newline for Firehose (common for downstream processing)
			data = append(data, '\n')
			recordSize := len(data)

			if len(batch) > 0 && (batchSize+recordSize > firehoseMaxBatchSize || len(batch) >= firehoseMaxRecords) {
				flush()
			}

			batch = append(batch, &firehose.Record{Data: data})
			batchSize += recordSize

		case <-ticker.C:
			flush()
		}
	}
}

func (f *Firehose) send(records []*firehose.Record) {
	resp, err := f.client.PutRecordBatch(&firehose.PutRecordBatchInput{
		DeliveryStreamName: aws.String(f.streamName),
		Records:            records,
	})
	if err != nil {
		slog.Error("firehose put failed", "error", err)
		return
	}

	if *resp.FailedPutCount > 0 {
		slog.Warn("firehose partial failure", "failed", *resp.FailedPutCount, "total", len(records))
	}
}
