#!/bin/bash
# Test: stdout output mode
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$SCRIPT_DIR/aws-lb-log-forwarder"
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT:-http://localhost:4566}"

BUCKET="e2e-stdout-test"

# Sample ALB log entries
read -r -d '' LOG_ENTRIES << 'EOF' || true
https 2024-03-21T10:15:30.123456Z app/my-alb/abc 192.168.1.100:54321 10.0.1.50:8080 0.001 0.015 0.000 200 200 256 1024 "GET https://api.example.com:443/users HTTP/1.1" "Mozilla/5.0" ECDHE-RSA-AES128-GCM-SHA256 TLSv1.2 arn:aws:elasticloadbalancing:us-east-1:123456789:tg/tg/abc "Root=1-abc" "api.example.com" "arn:aws:acm:us-east-1:123456789:cert/abc" 0 2024-03-21T10:15:30.107456Z "forward" "-" "-" "10.0.1.50:8080" "200" "-" "-" "-"
https 2024-03-21T10:15:31.234567Z app/my-alb/abc 192.168.1.101:54322 10.0.1.51:8080 0.002 0.020 0.001 404 404 512 2048 "POST https://api.example.com:443/data HTTP/1.1" "curl/7.88" ECDHE-RSA-AES256-GCM-SHA384 TLSv1.3 arn:aws:elasticloadbalancing:us-east-1:123456789:tg/tg/abc "Root=1-def" "api.example.com" "arn:aws:acm:us-east-1:123456789:cert/abc" 1 2024-03-21T10:15:31.212567Z "forward" "-" "-" "10.0.1.51:8080" "404" "-" "-" "-"
EOF

# Create gzipped log file
TEMP_LOG=$(mktemp)
TEMP_GZ="${TEMP_LOG}.gz"
echo "$LOG_ENTRIES" > "$TEMP_LOG"
gzip -c "$TEMP_LOG" > "$TEMP_GZ"

# Setup S3
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 mb "s3://$BUCKET" 2>/dev/null || true
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 cp "$TEMP_GZ" "s3://$BUCKET/logs/test.log.gz"

# Run forwarder with stdout output
export AWS_ENDPOINT_URL="$LOCALSTACK_ENDPOINT"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_REGION="eu-west-1"
export OUTPUTS="stdout"

OUTPUT=$("$BINARY" "s3://$BUCKET/logs/")

# Verify output contains expected data
if ! echo "$OUTPUT" | grep -q "elb_status_code"; then
    echo "ERROR: Output missing elb_status_code field"
    exit 1
fi

if ! echo "$OUTPUT" | grep -q "200"; then
    echo "ERROR: Output missing status code 200"
    exit 1
fi

LINE_COUNT=$(echo "$OUTPUT" | grep -c "time" || true)
if [ "$LINE_COUNT" -ne 2 ]; then
    echo "ERROR: Expected 2 log entries, got $LINE_COUNT"
    exit 1
fi

echo "stdout output verified: $LINE_COUNT entries"

# Cleanup
rm -f "$TEMP_LOG" "$TEMP_GZ"
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 rb "s3://$BUCKET" --force 2>/dev/null || true
