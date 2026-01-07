#!/bin/bash
# Test: NLB log processing
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$SCRIPT_DIR/aws-lb-log-forwarder"
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT:-http://localhost:4566}"

BUCKET="e2e-nlb-test"

# NOTE: Log entries must use recent timestamps. CloudWatch/LocalStack rejects events
# with timestamps too far in the past (>14 days). We generate timestamps dynamically.
NOW=$(date -u +"%Y-%m-%dT%H:%M:%S.000000Z")

# Sample NLB TLS log entry (24 fields - each field space-separated)
LOG_ENTRY="tls 2.0 ${NOW} net/my-nlb/abc123 g3e4-5678 192.168.1.100 54321 10.0.1.50 443 12 5000 52 1024 - arn:aws:elasticloadbalancing:us-east-1:123456789:cert/abc - ECDHE-RSA-AES128-GCM-SHA256 tlsv12 - my-nlb-abc.elb.us-east-1.amazonaws.com - - - ${NOW}"

# Create gzipped log file
TEMP_LOG=$(mktemp)
TEMP_GZ="${TEMP_LOG}.gz"
echo "$LOG_ENTRY" > "$TEMP_LOG"
gzip -c "$TEMP_LOG" > "$TEMP_GZ"

# Setup S3
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 mb "s3://$BUCKET" 2>/dev/null || true
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 cp "$TEMP_GZ" "s3://$BUCKET/logs/test.log.gz"

# Run forwarder in NLB mode
export AWS_ENDPOINT_URL="$LOCALSTACK_ENDPOINT"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_REGION="eu-west-1"
export OUTPUTS="stdout"
export LB_TYPE="nlb"

OUTPUT=$("$BINARY" "s3://$BUCKET/logs/")

# Verify NLB-specific fields are present
if ! echo "$OUTPUT" | grep -q "listener_id"; then
    echo "ERROR: Missing listener_id (NLB-specific field)"
    exit 1
fi

if ! echo "$OUTPUT" | grep -q "tls_cipher"; then
    echo "ERROR: Missing tls_cipher"
    exit 1
fi

echo "NLB log processing verified"

# Cleanup
rm -f "$TEMP_LOG" "$TEMP_GZ"
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 rb "s3://$BUCKET" --force 2>/dev/null || true
