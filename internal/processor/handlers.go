package processor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jdwit/alb-log-pipe/internal/types"
)

const maxConcurrency = 10

// HandleLambdaEvent processes S3 object creation events from Lambda.
func (p *LogProcessor) HandleLambdaEvent(ctx context.Context, event events.S3Event) error {
	objects := make([]types.S3ObjectInfo, 0, len(event.Records))
	for _, r := range event.Records {
		objects = append(objects, types.S3ObjectInfo{
			Bucket: r.S3.Bucket.Name,
			Key:    r.S3.Object.Key,
		})
	}
	return p.processObjects(ctx, objects)
}

// HandleS3URL processes all objects matching an S3 URL prefix (CLI mode).
func (p *LogProcessor) HandleS3URL(ctx context.Context, url string) error {
	bucket, prefix, err := parseS3URL(url)
	if err != nil {
		return fmt.Errorf("parse S3 URL: %w", err)
	}

	var objects []types.S3ObjectInfo
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

		for _, obj := range resp.Contents {
			objects = append(objects, types.S3ObjectInfo{
				Bucket: bucket,
				Key:    *obj.Key,
			})
		}

		if resp.IsTruncated == nil || !*resp.IsTruncated {
			break
		}
		token = resp.NextContinuationToken
	}

	return p.processObjects(ctx, objects)
}

func (p *LogProcessor) processObjects(ctx context.Context, objects []types.S3ObjectInfo) error {
	errs := make(chan error, len(objects))
	sem := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	for _, obj := range objects {
		wg.Add(1)
		sem <- struct{}{}

		go func(obj types.S3ObjectInfo) {
			defer func() {
				wg.Done()
				<-sem
			}()

			if err := p.ProcessLogs(ctx, obj); err != nil {
				errs <- fmt.Errorf("s3://%s/%s: %w", obj.Bucket, obj.Key, err)
			}
		}(obj)
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	var errList []error
	for err := range errs {
		errList = append(errList, err)
		slog.Error("processing failed", "error", err)
	}

	if len(errList) > 0 {
		return fmt.Errorf("%d objects failed to process", len(errList))
	}
	return nil
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
