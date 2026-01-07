//go:build e2e

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jdwit/alb-log-forwarder/internal/logprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	localstackEndpoint = "http://localhost:4566"
	testRegion         = "eu-west-1"
	testBucket         = "test-alb-logs"
	testLogGroup       = "/alb/test"
	testLogStream      = "e2e-test"
)

// Sample ALB log entries for e2e testing
var sampleLogs = `https 2024-03-21T10:15:30.123456Z app/my-alb/1234567890abcdef 192.168.1.100:54321 10.0.1.50:8080 0.001 0.015 0.000 200 200 256 1024 "GET https://api.example.com:443/v1/users?page=1 HTTP/1.1" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" ECDHE-RSA-AES128-GCM-SHA256 TLSv1.2 arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-targets/1234567890abcdef "Root=1-abcdef12-1234567890abcdef12345678" "api.example.com" "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012" 0 2024-03-21T10:15:30.107456Z "forward" "-" "-" "10.0.1.50:8080" "200" "-" "-" "-"
https 2024-03-21T10:15:31.234567Z app/my-alb/1234567890abcdef 192.168.1.101:54322 10.0.1.51:8080 0.002 0.020 0.001 201 201 512 2048 "POST https://api.example.com:443/v1/users HTTP/1.1" "axios/1.6.0" ECDHE-RSA-AES256-GCM-SHA384 TLSv1.3 arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-targets/1234567890abcdef "Root=1-abcdef13-1234567890abcdef12345679" "api.example.com" "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012" 1 2024-03-21T10:15:31.212567Z "forward" "-" "-" "10.0.1.51:8080" "201" "-" "-" "-"
https 2024-03-21T10:15:32.345678Z app/my-alb/1234567890abcdef 192.168.1.102:54323 10.0.1.52:8080 0.001 0.025 0.000 404 404 128 512 "GET https://api.example.com:443/v1/users/999 HTTP/1.1" "curl/7.88.1" ECDHE-RSA-AES128-GCM-SHA256 TLSv1.2 arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-targets/1234567890abcdef "Root=1-abcdef14-1234567890abcdef1234567a" "api.example.com" "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012" 2 2024-03-21T10:15:32.320678Z "forward" "-" "-" "10.0.1.52:8080" "404" "-" "-" "-"
`

func newLocalStackSession() *session.Session {
	return session.Must(session.NewSession(&aws.Config{
		Region:           aws.String(testRegion),
		Endpoint:         aws.String(localstackEndpoint),
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
		S3ForcePathStyle: aws.Bool(true),
	}))
}

func gzipBytes(data []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(data)
	gz.Close()
	return buf.Bytes()
}

func setupS3(t *testing.T, sess *session.Session) {
	t.Helper()
	s3Client := s3.New(sess)

	// Create bucket
	_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Logf("Bucket may already exist: %v", err)
	}

	// Upload gzipped log file
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String("logs/2024/03/21/test.log.gz"),
		Body:   bytes.NewReader(gzipBytes([]byte(sampleLogs))),
	})
	require.NoError(t, err)
}

func setupCloudWatch(t *testing.T, sess *session.Session) {
	t.Helper()
	cwClient := cloudwatchlogs.New(sess)

	// Create log group
	_, err := cwClient.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(testLogGroup),
	})
	if err != nil {
		t.Logf("Log group may already exist: %v", err)
	}

	// Create log stream
	_, err = cwClient.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(testLogGroup),
		LogStreamName: aws.String(testLogStream),
	})
	if err != nil {
		t.Logf("Log stream may already exist: %v", err)
	}
}

func getCloudWatchEvents(t *testing.T, sess *session.Session) []*cloudwatchlogs.OutputLogEvent {
	t.Helper()
	cwClient := cloudwatchlogs.New(sess)

	// Wait a bit for logs to be available
	time.Sleep(500 * time.Millisecond)

	resp, err := cwClient.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(testLogGroup),
		LogStreamName: aws.String(testLogStream),
		StartFromHead: aws.Bool(true),
	})
	require.NoError(t, err)
	return resp.Events
}

func TestE2E_LambdaEvent(t *testing.T) {
	sess := newLocalStackSession()

	// Setup
	setupS3(t, sess)
	setupCloudWatch(t, sess)

	// Configure environment
	os.Setenv("OUTPUTS", "cloudwatch")
	os.Setenv("CLOUDWATCH_LOG_GROUP", testLogGroup)
	os.Setenv("CLOUDWATCH_LOG_STREAM", testLogStream)
	os.Setenv("FIELDS", "")
	defer func() {
		os.Unsetenv("OUTPUTS")
		os.Unsetenv("CLOUDWATCH_LOG_GROUP")
		os.Unsetenv("CLOUDWATCH_LOG_STREAM")
		os.Unsetenv("FIELDS")
	}()

	// Create processor
	proc, err := logprocessor.New(sess)
	require.NoError(t, err)

	// Simulate S3 event
	event := events.S3Event{
		Records: []events.S3EventRecord{
			{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: testBucket},
					Object: events.S3Object{Key: "logs/2024/03/21/test.log.gz"},
				},
			},
		},
	}

	// Process
	err = proc.HandleLambdaEvent(context.Background(), event)
	require.NoError(t, err)

	// Verify logs in CloudWatch
	events := getCloudWatchEvents(t, sess)
	assert.Len(t, events, 3, "Expected 3 log entries in CloudWatch")
}

func TestE2E_CLIMode(t *testing.T) {
	sess := newLocalStackSession()

	// Setup - use different stream to avoid conflicts
	setupS3(t, sess)
	cwClient := cloudwatchlogs.New(sess)
	streamName := "e2e-cli-test"

	_, _ = cwClient.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(testLogGroup),
	})
	_, _ = cwClient.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(testLogGroup),
		LogStreamName: aws.String(streamName),
	})

	// Configure environment
	os.Setenv("OUTPUTS", "cloudwatch")
	os.Setenv("CLOUDWATCH_LOG_GROUP", testLogGroup)
	os.Setenv("CLOUDWATCH_LOG_STREAM", streamName)
	os.Setenv("FIELDS", "")
	defer func() {
		os.Unsetenv("OUTPUTS")
		os.Unsetenv("CLOUDWATCH_LOG_GROUP")
		os.Unsetenv("CLOUDWATCH_LOG_STREAM")
		os.Unsetenv("FIELDS")
	}()

	// Create processor
	proc, err := logprocessor.New(sess)
	require.NoError(t, err)

	// Process via S3 URL (CLI mode)
	err = proc.HandleS3URL(context.Background(), "s3://"+testBucket+"/logs/")
	require.NoError(t, err)

	// Verify logs in CloudWatch
	time.Sleep(500 * time.Millisecond)
	resp, err := cwClient.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(testLogGroup),
		LogStreamName: aws.String(streamName),
		StartFromHead: aws.Bool(true),
	})
	require.NoError(t, err)
	assert.Len(t, resp.Events, 3, "Expected 3 log entries in CloudWatch")
}

func TestE2E_FieldFiltering(t *testing.T) {
	sess := newLocalStackSession()

	// Setup
	setupS3(t, sess)
	cwClient := cloudwatchlogs.New(sess)
	streamName := "e2e-filter-test"

	_, _ = cwClient.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(testLogGroup),
	})
	_, _ = cwClient.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(testLogGroup),
		LogStreamName: aws.String(streamName),
	})

	// Configure with field filtering
	os.Setenv("OUTPUTS", "cloudwatch")
	os.Setenv("CLOUDWATCH_LOG_GROUP", testLogGroup)
	os.Setenv("CLOUDWATCH_LOG_STREAM", streamName)
	os.Setenv("FIELDS", "time,request,elb_status_code")
	defer func() {
		os.Unsetenv("OUTPUTS")
		os.Unsetenv("CLOUDWATCH_LOG_GROUP")
		os.Unsetenv("CLOUDWATCH_LOG_STREAM")
		os.Unsetenv("FIELDS")
	}()

	// Create processor
	proc, err := logprocessor.New(sess)
	require.NoError(t, err)

	// Process
	event := events.S3Event{
		Records: []events.S3EventRecord{
			{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: testBucket},
					Object: events.S3Object{Key: "logs/2024/03/21/test.log.gz"},
				},
			},
		},
	}
	err = proc.HandleLambdaEvent(context.Background(), event)
	require.NoError(t, err)

	// Verify logs contain only filtered fields
	time.Sleep(500 * time.Millisecond)
	resp, err := cwClient.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(testLogGroup),
		LogStreamName: aws.String(streamName),
		StartFromHead: aws.Bool(true),
	})
	require.NoError(t, err)
	require.Len(t, resp.Events, 3)

	// First event should only have 3 fields
	assert.Contains(t, *resp.Events[0].Message, "time")
	assert.Contains(t, *resp.Events[0].Message, "request")
	assert.Contains(t, *resp.Events[0].Message, "elb_status_code")
	assert.NotContains(t, *resp.Events[0].Message, "user_agent")
}
