#!/bin/bash
# Test: Field filtering
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$SCRIPT_DIR/aws-lb-log-forwarder"
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT:-http://localhost:4566}"

BUCKET="e2e-filter-test"

# Sample ALB log entry
LOG_ENTRY='https 2024-03-21T10:15:30.123456Z app/my-alb/abc 192.168.1.100:54321 10.0.1.50:8080 0.001 0.015 0.000 200 200 256 1024 "GET https://api.example.com:443/users HTTP/1.1" "Mozilla/5.0" ECDHE-RSA-AES128-GCM-SHA256 TLSv1.2 arn:aws:elasticloadbalancing:us-east-1:123456789:tg/tg/abc "Root=1-abc" "api.example.com" "arn:aws:acm:us-east-1:123456789:cert/abc" 0 2024-03-21T10:15:30.107456Z "forward" "-" "-" "10.0.1.50:8080" "200" "-" "-" "-"'

# Create gzipped log file
TEMP_LOG=$(mktemp)
TEMP_GZ="${TEMP_LOG}.gz"
echo "$LOG_ENTRY" > "$TEMP_LOG"
gzip -c "$TEMP_LOG" > "$TEMP_GZ"

# Setup S3
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 mb "s3://$BUCKET" 2>/dev/null || true
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 cp "$TEMP_GZ" "s3://$BUCKET/logs/test.log.gz"

# Run with field filtering
export AWS_ENDPOINT_URL="$LOCALSTACK_ENDPOINT"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_REGION="eu-west-1"
export OUTPUTS="stdout"
export FIELDS="time,elb_status_code,request"

OUTPUT=$("$BINARY" "s3://$BUCKET/logs/")

# Verify only selected fields are present
if ! echo "$OUTPUT" | grep -q "elb_status_code"; then
    echo "ERROR: Missing elb_status_code in filtered output"
    exit 1
fi

if ! echo "$OUTPUT" | grep -q "request"; then
    echo "ERROR: Missing request in filtered output"
    exit 1
fi

# Verify excluded fields are NOT present
if echo "$OUTPUT" | grep -q "user_agent"; then
    echo "ERROR: user_agent should be filtered out"
    exit 1
fi

if echo "$OUTPUT" | grep -q "trace_id"; then
    echo "ERROR: trace_id should be filtered out"
    exit 1
fi

echo "Field filtering verified"

# Cleanup
rm -f "$TEMP_LOG" "$TEMP_GZ"
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 rb "s3://$BUCKET" --force 2>/dev/null || true
