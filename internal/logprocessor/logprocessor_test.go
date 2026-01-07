package logprocessor

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jdwit/aws-lb-log-forwarder/internal/outputs"
	"github.com/jdwit/aws-lb-log-forwarder/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockS3API struct {
	mock.Mock
}

func (m *MockS3API) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func (m *MockS3API) ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.ListObjectsV2Output), args.Error(1)
}

func TestProcessLogs(t *testing.T) {
	t.Run("Successful Processing", func(t *testing.T) {
		mockS3 := new(MockS3API)

		mockBody := `https 2024-03-21T16:10:26.071854Z app/example-prod-lb/xxxxxxx4 192.0.2.104:36217 10.0.0.24:3003 0.004 0.024 0.003 203 203 1694 10783 "PUT https://example.com:443/api/modify?user_ids=xxxxx4-xxxx-xxxx-xxxx-xxxxxxxxxxxx&ref_date= HTTP/1.1" "axios/1.6.5" ECDHE-RSA-AES256-GCM-SHA384 TLSv1.3 arn:aws:elasticloadbalancing:xx-west-1:987654321098:targetgroup/example-prod-tg/xxxxxxxx4 "Root=1-xxxxxx4-xxxxxxxxxxxxxxxxxxxxxxxx" "example.com" "arn:aws:acm:xx-west-1:987654321098:certificate/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" 203 2024-03-21T16:10:26.061854Z "cache" "-" "-" "10.0.0.24:3003" "203" "-" "-" "TID_a1b2c3d4e5f67890abcdef1234567890"`

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write([]byte(mockBody))
		require.NoError(t, err)
		require.NoError(t, gz.Close())

		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(&buf),
		}, nil)

		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{outputs.NewStdout()},
		}

		err = lp.ProcessLogs(context.Background(), types.S3ObjectInfo{Bucket: "test-bucket", Key: "test-key"})
		require.NoError(t, err)
		mockS3.AssertExpectations(t)
	})
}

func TestParseRecords(t *testing.T) {
	t.Run("Process CSV Records", func(t *testing.T) {
		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{fields: fields}

		mockData := `https 2024-03-21T16:10:26.071854Z app/example-prod-lb/xxxxxxx4 192.0.2.104:36217 10.0.0.24:3003 0.004 0.024 0.003 203 203 1694 10783 "PUT https://example.com:443/api/modify?user_ids=xxxxx4-xxxx-xxxx-xxxx-xxxxxxxxxxxx&ref_date= HTTP/1.1" "axios/1.6.5" ECDHE-RSA-AES256-GCM-SHA384 TLSv1.3 arn:aws:elasticloadbalancing:xx-west-1:987654321098:targetgroup/example-prod-tg/xxxxxxxx4 "Root=1-xxxxxx4-xxxxxxxxxxxxxxxxxxxxxxxx" "example.com" "arn:aws:acm:xx-west-1:987654321098:certificate/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" 203 2024-03-21T16:10:26.061854Z "cache" "-" "-" "10.0.0.24:3003" "203" "-" "-" "TID_a1b2c3d4e5f67890abcdef1234567890"`

		entryChan := make(chan types.LogEntry, 10)

		go func() {
			err := lp.parseRecords(strings.NewReader(mockData), entryChan)
			require.NoError(t, err)
			close(entryChan)
		}()

		count := 0
		for entry := range entryChan {
			assert.Equal(t, "PUT https://example.com:443/api/modify?user_ids=xxxxx4-xxxx-xxxx-xxxx-xxxxxxxxxxxx&ref_date= HTTP/1.1", entry.Data["request"])
			count++
		}

		assert.Equal(t, 1, count)
	})
}

func TestRecordToEntry(t *testing.T) {
	t.Run("Valid Log Entry", func(t *testing.T) {
		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{fields: fields}

		record := []string{
			"https",
			"2024-03-21T16:10:26.071854Z",
			"app/example-prod-lb/xxxxxxx4",
			"192.0.2.104:36217",
			"10.0.0.24:3003",
			"0.004",
			"0.024",
			"0.003",
			"203",
			"203",
			"1694",
			"10783",
			"PUT https://example.com:443/api/modify?user_ids=xxxxx4-xxxx-xxxx-xxxx-xxxxxxxxxxxx&ref_date= HTTP/1.1",
			"axios/1.6.5",
			"ECDHE-RSA-AES256-GCM-SHA384",
			"TLSv1.3",
			"arn:aws:elasticloadbalancing:xx-west-1:987654321098:targetgroup/example-prod-tg/xxxxxxxx4",
			"Root=1-xxxxxx4-xxxxxxxxxxxxxxxxxxxxxxxx",
			"example.com",
			"arn:aws:acm:xx-west-1:987654321098:certificate/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			"203",
			"2024-03-21T16:10:26.061854Z",
			"cache",
			"-",
			"-",
			"10.0.0.24:3003",
			"203",
			"-",
			"-",
			"TID_a1b2c3d4e5f67890abcdef1234567890",
		}

		logEntry, err := lp.recordToEntry(record)
		require.NoError(t, err)
		assert.Equal(t, "2024-03-21T16:10:26.071854Z", logEntry.Timestamp.Format(time.RFC3339Nano))
		assert.Equal(t, "PUT https://example.com:443/api/modify?user_ids=xxxxx4-xxxx-xxxx-xxxx-xxxxxxxxxxxx&ref_date= HTTP/1.1", logEntry.Data["request"])
	})
}

func TestParseS3URL(t *testing.T) {
	t.Run("Valid S3 URL", func(t *testing.T) {
		bucket, key, err := parseS3URL("s3://mybucket/mykey")
		require.NoError(t, err)
		assert.Equal(t, "mybucket", bucket)
		assert.Equal(t, "mykey", key)
	})

	t.Run("Missing s3 prefix", func(t *testing.T) {
		_, _, err := parseS3URL("mybucket/mykey")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "s3://")
	})

	t.Run("No slash after bucket", func(t *testing.T) {
		_, _, err := parseS3URL("s3://mybucket")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "separator")
	})
}

// gzipData compresses data for mock S3 responses.
func gzipData(t *testing.T, data []byte) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(data)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	return &buf
}

// loadTestData reads and gzips the sample log file.
func loadTestData(t *testing.T) *bytes.Buffer {
	t.Helper()
	data, err := os.ReadFile("testdata/sample.log")
	require.NoError(t, err)
	return gzipData(t, data)
}

// MockOutput captures log entries for testing.
type MockOutput struct {
	mu      sync.Mutex
	entries []types.LogEntry
}

func (m *MockOutput) SendLogs(ctx context.Context, entries <-chan types.LogEntry) {
	for entry := range entries {
		m.mu.Lock()
		m.entries = append(m.entries, entry)
		m.mu.Unlock()
	}
}

func (m *MockOutput) Entries() []types.LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries
}

func TestHandleLambdaEvent(t *testing.T) {
	t.Run("Process single S3 event", func(t *testing.T) {
		mockS3 := new(MockS3API)
		mockOutput := &MockOutput{}

		buf := loadTestData(t)
		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(buf),
		}, nil)

		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{mockOutput},
		}

		event := events.S3Event{
			Records: []events.S3EventRecord{
				{
					S3: events.S3Entity{
						Bucket: events.S3Bucket{Name: "test-bucket"},
						Object: events.S3Object{Key: "logs/2024/03/21/test.log.gz"},
					},
				},
			},
		}

		err = lp.HandleLambdaEvent(context.Background(), event)
		require.NoError(t, err)
		mockS3.AssertExpectations(t)

		entries := mockOutput.Entries()
		assert.Len(t, entries, 5)
		assert.Equal(t, "200", entries[0].Data["elb_status_code"])
		assert.Equal(t, "201", entries[1].Data["elb_status_code"])
		assert.Equal(t, "404", entries[2].Data["elb_status_code"])
		assert.Equal(t, "500", entries[3].Data["elb_status_code"])
		assert.Equal(t, "204", entries[4].Data["elb_status_code"])
	})

	t.Run("Process multiple S3 events", func(t *testing.T) {
		mockS3 := new(MockS3API)
		mockOutput := &MockOutput{}

		// Each call needs its own buffer since it's consumed during read
		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(loadTestData(t)),
		}, nil).Once()
		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(loadTestData(t)),
		}, nil).Once()

		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{mockOutput},
		}

		event := events.S3Event{
			Records: []events.S3EventRecord{
				{
					S3: events.S3Entity{
						Bucket: events.S3Bucket{Name: "test-bucket"},
						Object: events.S3Object{Key: "logs/file1.log.gz"},
					},
				},
				{
					S3: events.S3Entity{
						Bucket: events.S3Bucket{Name: "test-bucket"},
						Object: events.S3Object{Key: "logs/file2.log.gz"},
					},
				},
			},
		}

		err = lp.HandleLambdaEvent(context.Background(), event)
		require.NoError(t, err)

		entries := mockOutput.Entries()
		assert.Len(t, entries, 10)
	})

	t.Run("S3 error handling", func(t *testing.T) {
		mockS3 := new(MockS3API)
		mockOutput := &MockOutput{}

		mockS3.On("GetObject", mock.Anything).Return(
			(*s3.GetObjectOutput)(nil),
			fmt.Errorf("access denied"),
		)

		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{mockOutput},
		}

		event := events.S3Event{
			Records: []events.S3EventRecord{
				{
					S3: events.S3Entity{
						Bucket: events.S3Bucket{Name: "test-bucket"},
						Object: events.S3Object{Key: "logs/test.log.gz"},
					},
				},
			},
		}

		err = lp.HandleLambdaEvent(context.Background(), event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})
}

func TestHandleS3URL(t *testing.T) {
	t.Run("Process objects matching prefix", func(t *testing.T) {
		mockS3 := new(MockS3API)
		mockOutput := &MockOutput{}

		mockS3.On("ListObjectsV2", mock.Anything).Return(&s3.ListObjectsV2Output{
			Contents: []*s3.Object{
				{Key: aws.String("logs/2024/03/21/file1.log.gz")},
				{Key: aws.String("logs/2024/03/21/file2.log.gz")},
			},
			IsTruncated: aws.Bool(false),
		}, nil)

		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(loadTestData(t)),
		}, nil).Once()
		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(loadTestData(t)),
		}, nil).Once()

		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{mockOutput},
		}

		err = lp.HandleS3URL(context.Background(), "s3://my-bucket/logs/2024/03/21/")
		require.NoError(t, err)

		entries := mockOutput.Entries()
		assert.Len(t, entries, 10)
	})

	t.Run("Paginated results", func(t *testing.T) {
		mockS3 := new(MockS3API)
		mockOutput := &MockOutput{}

		// First page
		mockS3.On("ListObjectsV2", mock.MatchedBy(func(input *s3.ListObjectsV2Input) bool {
			return input.ContinuationToken == nil
		})).Return(&s3.ListObjectsV2Output{
			Contents: []*s3.Object{
				{Key: aws.String("logs/file1.log.gz")},
			},
			IsTruncated:           aws.Bool(true),
			NextContinuationToken: aws.String("token1"),
		}, nil).Once()

		// Second page
		mockS3.On("ListObjectsV2", mock.MatchedBy(func(input *s3.ListObjectsV2Input) bool {
			return input.ContinuationToken != nil && *input.ContinuationToken == "token1"
		})).Return(&s3.ListObjectsV2Output{
			Contents: []*s3.Object{
				{Key: aws.String("logs/file2.log.gz")},
			},
			IsTruncated: aws.Bool(false),
		}, nil).Once()

		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(loadTestData(t)),
		}, nil).Once()
		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(loadTestData(t)),
		}, nil).Once()

		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{mockOutput},
		}

		err = lp.HandleS3URL(context.Background(), "s3://my-bucket/logs/")
		require.NoError(t, err)

		entries := mockOutput.Entries()
		assert.Len(t, entries, 10)
		mockS3.AssertExpectations(t)
	})

	t.Run("Invalid S3 URL", func(t *testing.T) {
		mockS3 := new(MockS3API)
		fields, _ := NewFieldFilter(LBTypeALB, "")

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{},
		}

		err := lp.HandleS3URL(context.Background(), "invalid-url")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "s3://")
	})
}

func TestProcessLogsWithRealisticData(t *testing.T) {
	t.Run("Full pipeline with sample logs", func(t *testing.T) {
		mockS3 := new(MockS3API)
		mockOutput := &MockOutput{}

		buf := loadTestData(t)
		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(buf),
		}, nil)

		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{mockOutput},
		}

		err = lp.ProcessLogs(context.Background(), types.S3ObjectInfo{
			Bucket: "test-bucket",
			Key:    "logs/test.log.gz",
		})
		require.NoError(t, err)

		entries := mockOutput.Entries()
		require.Len(t, entries, 5)

		// Verify first entry (GET request, 200)
		assert.Equal(t, "https", entries[0].Data["type"])
		assert.Equal(t, "app/my-alb/1234567890abcdef", entries[0].Data["elb"])
		assert.Equal(t, "192.168.1.100:54321", entries[0].Data["client:port"])
		assert.Equal(t, "200", entries[0].Data["elb_status_code"])
		assert.Contains(t, entries[0].Data["request"], "GET")
		assert.Contains(t, entries[0].Data["user_agent"], "Mozilla")

		// Verify timestamps are parsed correctly
		assert.Equal(t, 2024, entries[0].Timestamp.Year())
		assert.Equal(t, time.March, entries[0].Timestamp.Month())
		assert.Equal(t, 21, entries[0].Timestamp.Day())

		// Verify different HTTP methods
		assert.Contains(t, entries[0].Data["request"], "GET")
		assert.Contains(t, entries[1].Data["request"], "POST")
		assert.Contains(t, entries[3].Data["request"], "PUT")
		assert.Contains(t, entries[4].Data["request"], "DELETE")

		// Verify different status codes
		assert.Equal(t, "200", entries[0].Data["elb_status_code"])
		assert.Equal(t, "201", entries[1].Data["elb_status_code"])
		assert.Equal(t, "404", entries[2].Data["elb_status_code"])
		assert.Equal(t, "500", entries[3].Data["elb_status_code"])
		assert.Equal(t, "204", entries[4].Data["elb_status_code"])
	})

	t.Run("Field filtering", func(t *testing.T) {
		mockS3 := new(MockS3API)
		mockOutput := &MockOutput{}

		buf := loadTestData(t)
		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(buf),
		}, nil)

		fields, err := NewFieldFilter(LBTypeALB, "time,request,elb_status_code")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{mockOutput},
		}

		err = lp.ProcessLogs(context.Background(), types.S3ObjectInfo{
			Bucket: "test-bucket",
			Key:    "logs/test.log.gz",
		})
		require.NoError(t, err)

		entries := mockOutput.Entries()
		require.Len(t, entries, 5)

		// Only specified fields should be present
		assert.Len(t, entries[0].Data, 3)
		assert.Contains(t, entries[0].Data, "time")
		assert.Contains(t, entries[0].Data, "request")
		assert.Contains(t, entries[0].Data, "elb_status_code")
		assert.NotContains(t, entries[0].Data, "type")
		assert.NotContains(t, entries[0].Data, "client:port")
	})

	t.Run("Multiple outputs receive all entries", func(t *testing.T) {
		mockS3 := new(MockS3API)
		output1 := &MockOutput{}
		output2 := &MockOutput{}

		buf := loadTestData(t)
		mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{
			Body: io.NopCloser(buf),
		}, nil)

		fields, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		lp := &LogProcessor{
			s3:      mockS3,
			fields:  fields,
			outputs: []outputs.Output{output1, output2},
		}

		err = lp.ProcessLogs(context.Background(), types.S3ObjectInfo{
			Bucket: "test-bucket",
			Key:    "logs/test.log.gz",
		})
		require.NoError(t, err)

		// Both outputs should receive ALL entries (fan-out replication)
		entries1 := output1.Entries()
		entries2 := output2.Entries()
		assert.Len(t, entries1, 5, "output1 should receive all 5 entries")
		assert.Len(t, entries2, 5, "output2 should receive all 5 entries")

		// Verify both received the same data
		for i := 0; i < 5; i++ {
			assert.Equal(t, entries1[i].Data["elb_status_code"], entries2[i].Data["elb_status_code"],
				"entry %d should have same status code in both outputs", i)
		}
	})
}
