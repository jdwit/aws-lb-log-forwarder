package outputs

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
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
