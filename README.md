# ALB Log Forwarder

Forward AWS Application Load Balancer access logs to CloudWatch Logs.

```mermaid
flowchart LR
    ALB[Application Load Balancer] --> S3[(S3 Bucket)]
    S3 -->|ObjectCreated| Lambda[ALB Log Forwarder]
    Lambda --> CW[CloudWatch Logs]
    Lambda --> OUT[stdout]
```

## Overview

ALBs store access logs as gzip-compressed files in S3. ALB Log Forwarder runs as a Lambda function that automatically processes new log files and forwards them to CloudWatch Logs or stdout.

## Installation

### CLI

```bash
go install github.com/jdwit/alb-log-forwarder@latest
```

### Lambda

```bash
# Build for Lambda
GOOS=linux GOARCH=amd64 go build -o bootstrap .
zip lambda.zip bootstrap

# Upload lambda.zip to AWS Lambda:
# - Runtime: Amazon Linux 2023 (provided.al2023)
# - Handler: bootstrap
# - Trigger: S3 ObjectCreated events on your ALB logs bucket
```

## Configuration

Environment variables:

| Variable | Description | Required |
|----------|-------------|----------|
| `TARGETS` | Comma-separated: `cloudwatch`, `stdout` | Yes |
| `CLOUDWATCH_LOG_GROUP` | CloudWatch Log Group name | If cloudwatch |
| `CLOUDWATCH_LOG_STREAM` | CloudWatch Log Stream name | If cloudwatch |
| `FIELDS` | Comma-separated fields to include (empty = all) | No |

### Fields

All 30 ALB log fields are supported. See [ALB access log entries](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html#access-log-entry-format).

Common: `type`, `time`, `elb`, `client:port`, `target:port`, `request_processing_time`, `target_processing_time`, `response_processing_time`, `elb_status_code`, `target_status_code`, `request`, `user_agent`

## CLI Usage

```bash
# Output to stdout
TARGETS=stdout ./alb-log-forwarder s3://bucket/AWSLogs/123456789/elasticloadbalancing/us-east-1/2024/01/01/

# Forward to CloudWatch with field filtering
TARGETS=cloudwatch \
CLOUDWATCH_LOG_GROUP=/alb/logs \
CLOUDWATCH_LOG_STREAM=prod \
FIELDS=request,elb_status_code,target_processing_time \
./alb-log-forwarder s3://bucket/AWSLogs/123456789/elasticloadbalancing/us-east-1/2024/01/01/
```

## License

MIT
