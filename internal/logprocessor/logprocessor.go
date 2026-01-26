package logprocessor

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jdwit/aws-lb-log-forwarder/internal/destinations"
	"github.com/jdwit/aws-lb-log-forwarder/internal/types"
	"golang.org/x/sync/errgroup"
)

const (
	maxConcurrency    = 10
	defaultBufferSize = 2000
)

// S3API defines the S3 operations used by LogProcessor.
type S3API interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
}

// LogProcessor processes load balancer log files from S3 and sends them to configured destinations.
type LogProcessor struct {
	s3           S3API
	fields       *FieldFilter
	destinations []destinations.Destination
	bufferSize   int
}

// New creates a LogProcessor from environment configuration.
func New(sess *session.Session) (*LogProcessor, error) {
	lbType := LBType(strings.ToLower(os.Getenv("LB_TYPE")))
	if lbType == "" {
		lbType = LBTypeALB // default to ALB for backwards compatibility
	}

	fields, err := NewFieldFilter(lbType, os.Getenv("FIELDS"))
	if err != nil {
		return nil, fmt.Errorf("invalid fields config: %w", err)
	}

	dests, err := destinations.New(os.Getenv("DESTINATIONS"), sess)
	if err != nil {
		return nil, fmt.Errorf("invalid destinations config: %w", err)
	}

	bufferSize := defaultBufferSize
	if v := os.Getenv("BUFFER_SIZE"); v != "" {
		bufferSize, err = strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid BUFFER_SIZE: %w", err)
		}
	}

	return &LogProcessor{
		s3:           s3.New(sess),
		fields:       fields,
		destinations: dests,
		bufferSize:   bufferSize,
	}, nil
}

// NewWithDeps creates a LogProcessor with explicit dependencies (for testing).
func NewWithDeps(s3Client S3API, fields *FieldFilter, dests []destinations.Destination) *LogProcessor {
	if fields == nil {
		fields, _ = NewFieldFilter(LBTypeALB, "")
	}
	return &LogProcessor{s3: s3Client, fields: fields, destinations: dests, bufferSize: defaultBufferSize}
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

	// Create a channel per destination for fan-out (each destination receives all entries)
	channels := make([]chan types.LogEntry, len(p.destinations))
	var wg sync.WaitGroup
	for i, d := range p.destinations {
		ch := make(chan types.LogEntry, p.bufferSize)
		channels[i] = ch
		wg.Add(1)
		go func(d destinations.Destination, ch <-chan types.LogEntry) {
			defer wg.Done()
			d.SendLogs(ctx, ch)
		}(d, ch)
	}

	// Parse records and fan out to all destination channels
	entries := make(chan types.LogEntry, p.bufferSize)
	go func() {
		if err := p.parseRecords(pr, entries); err != nil {
			slog.Error("parse failed", "error", err)
		}
		close(entries)
	}()

	// Fan out: send each entry to all destination channels
	var count int
	for entry := range entries {
		count++
		for _, ch := range channels {
			ch <- entry
		}
	}

	// Close all destination channels
	for _, ch := range channels {
		close(ch)
	}
	wg.Wait()

	slog.Info("completed", "bucket", obj.Bucket, "key", obj.Key, "entries", count)
	return nil
}

func (p *LogProcessor) parseRecords(r io.Reader, out chan<- types.LogEntry) error {
	cr := csv.NewReader(r)
	cr.Comma = ' '
	cr.FieldsPerRecord = -1 // Allow variable field count for forward compatibility

	expectedFields := p.fields.TotalFields()
	extraFieldsLogged := false

	for {
		record, err := cr.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read record: %w", err)
		}

		if !extraFieldsLogged && len(record) > expectedFields {
			slog.Warn("log format has more fields than expected, new fields may have been added",
				"expected", expectedFields, "got", len(record))
			extraFieldsLogged = true
		}

		entry, err := p.recordToEntry(record)
		if err != nil {
			return err
		}
		out <- entry
	}
}

func (p *LogProcessor) recordToEntry(record []string) (types.LogEntry, error) {
	minFields := p.fields.TotalFields()
	if len(record) < minFields {
		return types.LogEntry{}, fmt.Errorf("expected at least %d fields, got %d", minFields, len(record))
	}

	// Time field is at index 1 for ALB, index 2 for NLB
	timeIdx := 1
	if p.fields.LBType() == LBTypeNLB {
		timeIdx = 2
	}

	ts, err := time.Parse(time.RFC3339, record[timeIdx])
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
