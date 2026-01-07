package outputs

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/jdwit/alb-log-forwarder/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockFirehoseClient struct {
	mock.Mock
}

func (m *MockFirehoseClient) PutRecordBatch(input *firehose.PutRecordBatchInput) (*firehose.PutRecordBatchOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*firehose.PutRecordBatchOutput), args.Error(1)
}

func TestFirehose_Send(t *testing.T) {
	t.Run("Send records successfully", func(t *testing.T) {
		mockClient := new(MockFirehoseClient)
		mockClient.On("PutRecordBatch", mock.Anything).Return(&firehose.PutRecordBatchOutput{
			FailedPutCount: aws.Int64(0),
		}, nil)

		records := []*firehose.Record{
			{Data: []byte(`{"message":"test1"}` + "\n")},
			{Data: []byte(`{"message":"test2"}` + "\n")},
		}

		fh := &Firehose{
			client:     mockClient,
			streamName: "test-stream",
		}

		fh.send(records)
		mockClient.AssertExpectations(t)
	})
}

func TestFirehose_SendLogs(t *testing.T) {
	t.Run("Process entries", func(t *testing.T) {
		mockClient := new(MockFirehoseClient)
		mockClient.On("PutRecordBatch", mock.Anything).Return(&firehose.PutRecordBatchOutput{
			FailedPutCount: aws.Int64(0),
		}, nil)

		fh := &Firehose{
			client:     mockClient,
			streamName: "test-stream",
		}

		entries := make(chan types.LogEntry, 2)
		entries <- types.LogEntry{
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "test1"},
		}
		entries <- types.LogEntry{
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "test2"},
		}
		close(entries)

		fh.SendLogs(context.Background(), entries)
		mockClient.AssertExpectations(t)
	})
}

func TestNewFirehose(t *testing.T) {
	t.Run("Missing stream name", func(t *testing.T) {
		t.Setenv("FIREHOSE_STREAM_NAME", "")
		_, err := NewFirehose(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "FIREHOSE_STREAM_NAME")
	})
}
