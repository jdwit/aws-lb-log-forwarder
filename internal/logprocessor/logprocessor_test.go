package logprocessor

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jdwit/alb-log-forwarder/internal/outputs"
	"github.com/jdwit/alb-log-forwarder/internal/types"
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

		fields, err := NewFieldFilter("")
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
		fields, err := NewFieldFilter("")
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
		fields, err := NewFieldFilter("")
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
