package processor

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jdwit/alb-log-forwarder/internal/targets"
	"github.com/jdwit/alb-log-forwarder/internal/types"
)

// S3API defines the S3 operations used by LogProcessor.
type S3API interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
}

// LogProcessor processes ALB log files from S3 and sends them to configured targets.
type LogProcessor struct {
	s3      S3API
	fields  *Fields
	targets []targets.Target
}

// New creates a LogProcessor from environment configuration.
func New(sess *session.Session) (*LogProcessor, error) {
	fields, err := NewFields(os.Getenv("FIELDS"))
	if err != nil {
		return nil, fmt.Errorf("invalid fields config: %w", err)
	}

	tgts, err := targets.New(os.Getenv("TARGETS"), sess)
	if err != nil {
		return nil, fmt.Errorf("invalid targets config: %w", err)
	}

	return &LogProcessor{
		s3:      s3.New(sess),
		fields:  fields,
		targets: tgts,
	}, nil
}

// ProcessLogs downloads and processes a single S3 object.
func (p *LogProcessor) ProcessLogs(ctx context.Context, obj types.S3ObjectInfo) error {
	slog.Info("processing", "bucket", obj.Bucket, "key", obj.Key)

	resp, err := p.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(obj.Bucket),
		Key:    aws.String(obj.Key),
	})
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer resp.Body.Close()

	pr, pw := io.Pipe()

	// Decompress in background
	go func() {
		defer pw.Close()
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("gzip reader: %w", err))
			return
		}
		defer gr.Close()
		if _, err := io.Copy(pw, gr); err != nil {
			pw.CloseWithError(fmt.Errorf("decompress: %w", err))
		}
	}()

	entries := make(chan types.LogEntry, 12500)

	var wg sync.WaitGroup
	for _, t := range p.targets {
		wg.Add(1)
		go func(t targets.Target) {
			defer wg.Done()
			t.SendLogs(ctx, entries)
		}(t)
	}

	if err := p.parseRecords(pr, entries); err != nil {
		slog.Error("parse failed", "error", err)
	}
	close(entries)
	wg.Wait()

	slog.Info("completed", "bucket", obj.Bucket, "key", obj.Key)
	return nil
}

func (p *LogProcessor) parseRecords(r io.Reader, out chan<- types.LogEntry) error {
	cr := csv.NewReader(r)
	cr.Comma = ' '
	cr.FieldsPerRecord = FieldCount()

	for {
		record, err := cr.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read record: %w", err)
		}

		entry, err := p.recordToEntry(record)
		if err != nil {
			return err
		}
		out <- entry
	}
}

func (p *LogProcessor) recordToEntry(record []string) (types.LogEntry, error) {
	if len(record) != FieldCount() {
		return types.LogEntry{}, fmt.Errorf("expected %d fields, got %d", FieldCount(), len(record))
	}

	ts, err := time.Parse(time.RFC3339, record[1])
	if err != nil {
		return types.LogEntry{}, fmt.Errorf("parse timestamp: %w", err)
	}

	data := make(map[string]string)
	for i, val := range record {
		if p.fields.Include(i) {
			name, _ := p.fields.FieldName(i)
			data[name] = val
		}
	}

	return types.LogEntry{Data: data, Timestamp: ts}, nil
}
