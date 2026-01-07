package logprocessor

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jdwit/alb-log-forwarder/internal/outputs"
	"github.com/jdwit/alb-log-forwarder/internal/types"
	"golang.org/x/sync/errgroup"
)

const maxConcurrency = 10

// S3API defines the S3 operations used by LogProcessor.
type S3API interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
}

// LogProcessor processes ALB log files from S3 and sends them to configured outputs.
type LogProcessor struct {
	s3      S3API
	fields  *FieldFilter
	outputs []outputs.Output
}

// New creates a LogProcessor from environment configuration.
func New(sess *session.Session) (*LogProcessor, error) {
	fields, err := NewFieldFilter(os.Getenv("FIELDS"))
	if err != nil {
		return nil, fmt.Errorf("invalid fields config: %w", err)
	}

	outs, err := outputs.New(os.Getenv("OUTPUTS"), sess)
	if err != nil {
		return nil, fmt.Errorf("invalid outputs config: %w", err)
	}

	return &LogProcessor{
		s3:      s3.New(sess),
		fields:  fields,
		outputs: outs,
	}, nil
}

// HandleLambdaEvent processes S3 object creation events from Lambda.
func (p *LogProcessor) HandleLambdaEvent(ctx context.Context, event events.S3Event) error {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	for _, r := range event.Records {
		obj := types.S3ObjectInfo{
			Bucket: r.S3.Bucket.Name,
			Key:    r.S3.Object.Key,
		}
		g.Go(func() error {
			return p.processObject(ctx, obj)
		})
	}

	return g.Wait()
}

// HandleS3URL processes all objects matching an S3 URL prefix (CLI mode).
func (p *LogProcessor) HandleS3URL(ctx context.Context, url string) error {
	bucket, prefix, err := parseS3URL(url)
	if err != nil {
		return fmt.Errorf("parse S3 URL: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	var token *string
	for {
		resp, err := p.s3.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return fmt.Errorf("list objects: %w", err)
		}

		for _, item := range resp.Contents {
			obj := types.S3ObjectInfo{
				Bucket: bucket,
				Key:    *item.Key,
			}
			g.Go(func() error {
				return p.processObject(ctx, obj)
			})
		}

		if resp.IsTruncated == nil || !*resp.IsTruncated {
			break
		}
		token = resp.NextContinuationToken
	}

	return g.Wait()
}

func (p *LogProcessor) processObject(ctx context.Context, obj types.S3ObjectInfo) error {
	if err := p.ProcessLogs(ctx, obj); err != nil {
		err = fmt.Errorf("s3://%s/%s: %w", obj.Bucket, obj.Key, err)
		slog.Error("processing failed", "error", err)
		return err
	}
	return nil
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
	for _, o := range p.outputs {
		wg.Add(1)
		go func(o outputs.Output) {
			defer wg.Done()
			o.SendLogs(ctx, entries)
		}(o)
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
	cr.FieldsPerRecord = TotalFields()

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
	if len(record) != TotalFields() {
		return types.LogEntry{}, fmt.Errorf("expected %d fields, got %d", TotalFields(), len(record))
	}

	ts, err := time.Parse(time.RFC3339, record[1])
	if err != nil {
		return types.LogEntry{}, fmt.Errorf("parse timestamp: %w", err)
	}

	data := make(map[string]string)
	for i, val := range record {
		if p.fields.Includes(i) {
			name, _ := p.fields.Name(i)
			data[name] = val
		}
	}

	return types.LogEntry{Data: data, Timestamp: ts}, nil
}

func parseS3URL(url string) (bucket, prefix string, err error) {
	if !strings.HasPrefix(url, "s3://") {
		return "", "", fmt.Errorf("must start with s3://")
	}

	path := strings.TrimPrefix(url, "s3://")
	idx := strings.Index(path, "/")
	if idx == -1 {
		return "", "", fmt.Errorf("missing path separator")
	}

	return path[:idx], path[idx+1:], nil
}
