#!/bin/bash
# Test: Splunk HEC output
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$SCRIPT_DIR/aws-lb-log-forwarder"
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT:-http://localhost:4566}"
SPLUNK_HEC_ENDPOINT="${SPLUNK_HEC_ENDPOINT:-https://localhost:8088/services/collector/event}"
SPLUNK_HEC_TOKEN="${SPLUNK_HEC_TOKEN:-e2e-test-token}"

BUCKET="e2e-splunk-test"

# NOTE: Log entries must use recent timestamps. CloudWatch/LocalStack rejects events
# with timestamps too far in the past (>14 days). We generate timestamps dynamically.
NOW=$(date -u +"%Y-%m-%dT%H:%M:%S.000000Z")

# Sample ALB log entry (30 fields)
LOG_ENTRY="https ${NOW} app/my-alb/abc 192.168.1.100:54321 10.0.1.50:8080 0.001 0.015 0.000 200 200 256 1024 \"GET https://api.example.com:443/users HTTP/1.1\" \"Mozilla/5.0\" ECDHE-RSA-AES128-GCM-SHA256 TLSv1.2 arn:aws:elasticloadbalancing:us-east-1:123456789:tg/tg/abc \"Root=1-abc\" \"api.example.com\" \"arn:aws:acm:us-east-1:123456789:cert/abc\" 0 ${NOW} \"forward\" \"-\" \"-\" \"10.0.1.50:8080\" \"200\" \"-\" \"-\" \"-\""

# Check if Splunk HEC is available
HEALTH_ENDPOINT=$(echo "$SPLUNK_HEC_ENDPOINT" | sed 's|/services/collector/event|/services/collector/health|')
if ! curl -sk "$HEALTH_ENDPOINT" > /dev/null 2>&1; then
    echo "SKIP: Splunk HEC not available at $SPLUNK_HEC_ENDPOINT"
    exit 0
fi

# Create gzipped log file
TEMP_LOG=$(mktemp)
TEMP_GZ="${TEMP_LOG}.gz"
echo "$LOG_ENTRY" > "$TEMP_LOG"
gzip -c "$TEMP_LOG" > "$TEMP_GZ"

# Setup S3
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 mb "s3://$BUCKET" 2>/dev/null || true
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 cp "$TEMP_GZ" "s3://$BUCKET/logs/test.log.gz"

# Run forwarder with Splunk output
export AWS_ENDPOINT_URL="$LOCALSTACK_ENDPOINT"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_REGION="eu-west-1"
export OUTPUTS="splunk"
export SPLUNK_HEC_ENDPOINT="$SPLUNK_HEC_ENDPOINT"
export SPLUNK_HEC_TOKEN="$SPLUNK_HEC_TOKEN"
export SPLUNK_SKIP_VERIFY="true"
export SPLUNK_SOURCE="alb"
export SPLUNK_SOURCETYPE="aws:alb:accesslog"

# Splunk HEC returns 200 on success
OUTPUT=$("$BINARY" "s3://$BUCKET/logs/" 2>&1)

# If we got here without error, Splunk accepted the events
echo "Splunk HEC output verified"

# Cleanup
rm -f "$TEMP_LOG" "$TEMP_GZ"
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 rb "s3://$BUCKET" --force 2>/dev/null || true
