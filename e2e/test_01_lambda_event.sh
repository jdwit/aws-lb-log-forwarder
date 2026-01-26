#!/bin/bash
# Test: Lambda event processing with CloudWatch destination
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$SCRIPT_DIR/aws-lb-log-forwarder"
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT:-http://localhost:4566}"

BUCKET="e2e-test-bucket"
LOG_GROUP="/alb/e2e-test"
LOG_STREAM="test-lambda-event"

# NOTE: Log entries must use recent timestamps. CloudWatch/LocalStack rejects events
# with timestamps too far in the past (>14 days). We generate timestamps dynamically.
NOW=$(date -u +"%Y-%m-%dT%H:%M:%S.000000Z")

# Sample ALB log entry (30 fields)
LOG_ENTRY="https ${NOW} app/my-alb/1234567890abcdef 192.168.1.100:54321 10.0.1.50:8080 0.001 0.015 0.000 200 200 256 1024 \"GET https://api.example.com:443/v1/users?page=1 HTTP/1.1\" \"Mozilla/5.0\" ECDHE-RSA-AES128-GCM-SHA256 TLSv1.2 arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-targets/1234567890abcdef \"Root=1-abc\" \"api.example.com\" \"arn:aws:acm:us-east-1:123456789012:certificate/abc\" 0 ${NOW} \"forward\" \"-\" \"-\" \"10.0.1.50:8080\" \"200\" \"-\" \"-\" \"-\""

# Create gzipped log file
TEMP_LOG=$(mktemp)
TEMP_GZ="${TEMP_LOG}.gz"
echo "$LOG_ENTRY" > "$TEMP_LOG"
gzip -c "$TEMP_LOG" > "$TEMP_GZ"

# Setup S3
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 mb "s3://$BUCKET" 2>/dev/null || true
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 cp "$TEMP_GZ" "s3://$BUCKET/logs/test.log.gz"

# Setup CloudWatch
aws --endpoint-url="$LOCALSTACK_ENDPOINT" logs create-log-group --log-group-name "$LOG_GROUP" 2>/dev/null || true
aws --endpoint-url="$LOCALSTACK_ENDPOINT" logs create-log-stream --log-group-name "$LOG_GROUP" --log-stream-name "$LOG_STREAM" 2>/dev/null || true

# Run forwarder in CLI mode (simulates what Lambda does)
export AWS_ENDPOINT_URL="$LOCALSTACK_ENDPOINT"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_REGION="eu-west-1"
export DESTINATIONS="cloudwatch"
export CLOUDWATCH_LOG_GROUP="$LOG_GROUP"
export CLOUDWATCH_LOG_STREAM="$LOG_STREAM"

"$BINARY" "s3://$BUCKET/logs/"

# Verify logs in CloudWatch
sleep 1
EVENTS=$(aws --endpoint-url="$LOCALSTACK_ENDPOINT" logs get-log-events \
    --log-group-name "$LOG_GROUP" \
    --log-stream-name "$LOG_STREAM" \
    --query 'events[*].message' --output text)

if [ -z "$EVENTS" ]; then
    echo "ERROR: No events found in CloudWatch"
    exit 1
fi

echo "Found log events in CloudWatch"

# Cleanup
rm -f "$TEMP_LOG" "$TEMP_GZ"
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 rb "s3://$BUCKET" --force 2>/dev/null || true
