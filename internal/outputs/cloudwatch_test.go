package outputs

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/jdwit/alb-log-forwarder/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockCloudWatchClient struct {
	mock.Mock
}

func (m *MockCloudWatchClient) PutLogEvents(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.PutLogEventsOutput), args.Error(1)
}

func (m *MockCloudWatchClient) CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.CreateLogGroupOutput), args.Error(1)
}

func (m *MockCloudWatchClient) CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.CreateLogStreamOutput), args.Error(1)
}

func (m *MockCloudWatchClient) DescribeLogGroups(input *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.DescribeLogGroupsOutput), args.Error(1)
}

func (m *MockCloudWatchClient) DescribeLogStreams(input *cloudwatchlogs.DescribeLogStreamsInput) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.DescribeLogStreamsOutput), args.Error(1)
}

func TestEnsureLogGroup(t *testing.T) {
	t.Run("Log group exists", func(t *testing.T) {
		mockClient := new(MockCloudWatchClient)
		mockClient.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{
				{LogGroupName: aws.String("test-log-group")},
			},
		}, nil)

		err := ensureLogGroup(mockClient, "test-log-group")
		require.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("Log group does not exist", func(t *testing.T) {
		mockClient := new(MockCloudWatchClient)
		mockClient.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{},
		}, nil)

		mockClient.On("CreateLogGroup", &cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: aws.String("test-log-group"),
		}).Return(&cloudwatchlogs.CreateLogGroupOutput{}, nil)

		err := ensureLogGroup(mockClient, "test-log-group")
		require.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}

func TestEnsureLogStream(t *testing.T) {
	t.Run("Log stream exists", func(t *testing.T) {
		mockClient := new(MockCloudWatchClient)
		mockClient.On("DescribeLogStreams", mock.Anything).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
			LogStreams: []*cloudwatchlogs.LogStream{
				{LogStreamName: aws.String("test-log-stream")},
			},
		}, nil)

		err := ensureLogStream(mockClient, "test-log-group", "test-log-stream")
		require.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("Log stream does not exist", func(t *testing.T) {
		mockClient := new(MockCloudWatchClient)
		mockClient.On("DescribeLogStreams", mock.Anything).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
			LogStreams: []*cloudwatchlogs.LogStream{},
		}, nil)

		mockClient.On("CreateLogStream", &cloudwatchlogs.CreateLogStreamInput{
			LogGroupName:  aws.String("test-log-group"),
			LogStreamName: aws.String("test-log-stream"),
		}).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil)

		err := ensureLogStream(mockClient, "test-log-group", "test-log-stream")
		require.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}

func TestSend(t *testing.T) {
	t.Run("Send events successfully", func(t *testing.T) {
		mockClient := new(MockCloudWatchClient)
		mockClient.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)

		events := []*cloudwatchlogs.InputLogEvent{
			{Message: aws.String("message1"), Timestamp: aws.Int64(1)},
			{Message: aws.String("message2"), Timestamp: aws.Int64(2)},
		}

		cw := &CloudWatch{
			client:    mockClient,
			logGroup:  "test-log-group",
			logStream: "test-log-stream",
		}

		cw.send(events)
		mockClient.AssertExpectations(t)
	})
}

func TestCloudWatch_SendLogs(t *testing.T) {
	t.Run("Process entries from channel", func(t *testing.T) {
		mockClient := new(MockCloudWatchClient)
		mockClient.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)

		cw := &CloudWatch{
			client:    mockClient,
			logGroup:  "test-group",
			logStream: "test-stream",
		}

		entries := make(chan types.LogEntry, 3)
		entries <- types.LogEntry{
			Timestamp: time.Date(2024, 3, 21, 10, 15, 30, 0, time.UTC),
			Data:      map[string]string{"request": "GET /api/users", "status": "200"},
		}
		entries <- types.LogEntry{
			Timestamp: time.Date(2024, 3, 21, 10, 15, 31, 0, time.UTC),
			Data:      map[string]string{"request": "POST /api/users", "status": "201"},
		}
		entries <- types.LogEntry{
			Timestamp: time.Date(2024, 3, 21, 10, 15, 32, 0, time.UTC),
			Data:      map[string]string{"request": "DELETE /api/users/1", "status": "204"},
		}
		close(entries)

		cw.SendLogs(context.Background(), entries)

		mockClient.AssertCalled(t, "PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
			return *input.LogGroupName == "test-group" &&
				*input.LogStreamName == "test-stream" &&
				len(input.LogEvents) == 3
		}))
	})

	t.Run("Context cancellation flushes remaining", func(t *testing.T) {
		mockClient := new(MockCloudWatchClient)
		mockClient.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)

		cw := &CloudWatch{
			client:    mockClient,
			logGroup:  "test-group",
			logStream: "test-stream",
		}

		ctx, cancel := context.WithCancel(context.Background())
		entries := make(chan types.LogEntry, 1)
		entries <- types.LogEntry{
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "test"},
		}

		done := make(chan struct{})
		go func() {
			cw.SendLogs(ctx, entries)
			close(done)
		}()

		cancel()
		<-done

		mockClient.AssertCalled(t, "PutLogEvents", mock.Anything)
	})

	t.Run("Events are sorted by timestamp", func(t *testing.T) {
		mockClient := new(MockCloudWatchClient)
		var capturedInput *cloudwatchlogs.PutLogEventsInput
		mockClient.On("PutLogEvents", mock.Anything).Run(func(args mock.Arguments) {
			capturedInput = args.Get(0).(*cloudwatchlogs.PutLogEventsInput)
		}).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)

		cw := &CloudWatch{
			client:    mockClient,
			logGroup:  "test-group",
			logStream: "test-stream",
		}

		entries := make(chan types.LogEntry, 3)
		// Send out of order
		entries <- types.LogEntry{
			Timestamp: time.Date(2024, 3, 21, 10, 15, 32, 0, time.UTC),
			Data:      map[string]string{"order": "third"},
		}
		entries <- types.LogEntry{
			Timestamp: time.Date(2024, 3, 21, 10, 15, 30, 0, time.UTC),
			Data:      map[string]string{"order": "first"},
		}
		entries <- types.LogEntry{
			Timestamp: time.Date(2024, 3, 21, 10, 15, 31, 0, time.UTC),
			Data:      map[string]string{"order": "second"},
		}
		close(entries)

		cw.SendLogs(context.Background(), entries)

		require.NotNil(t, capturedInput)
		require.Len(t, capturedInput.LogEvents, 3)

		// Verify events are sorted by timestamp
		for i := 1; i < len(capturedInput.LogEvents); i++ {
			assert.LessOrEqual(t, *capturedInput.LogEvents[i-1].Timestamp, *capturedInput.LogEvents[i].Timestamp)
		}
	})
}
