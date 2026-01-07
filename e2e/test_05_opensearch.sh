#!/bin/bash
# Test: OpenSearch output
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$SCRIPT_DIR/aws-lb-log-forwarder"
LOCALSTACK_ENDPOINT="${LOCALSTACK_ENDPOINT:-http://localhost:4566}"
OPENSEARCH_ENDPOINT="${OPENSEARCH_ENDPOINT:-http://localhost:9200}"

BUCKET="e2e-opensearch-test"
INDEX="alb-logs-test"

# NOTE: Log entries must use recent timestamps. CloudWatch/LocalStack rejects events
# with timestamps too far in the past (>14 days). We generate timestamps dynamically.
NOW=$(date -u +"%Y-%m-%dT%H:%M:%S.000000Z")

# Sample ALB log entry (30 fields)
LOG_ENTRY="https ${NOW} app/my-alb/abc 192.168.1.100:54321 10.0.1.50:8080 0.001 0.015 0.000 200 200 256 1024 \"GET https://api.example.com:443/users HTTP/1.1\" \"Mozilla/5.0\" ECDHE-RSA-AES128-GCM-SHA256 TLSv1.2 arn:aws:elasticloadbalancing:us-east-1:123456789:tg/tg/abc \"Root=1-abc\" \"api.example.com\" \"arn:aws:acm:us-east-1:123456789:cert/abc\" 0 ${NOW} \"forward\" \"-\" \"-\" \"10.0.1.50:8080\" \"200\" \"-\" \"-\" \"-\""

# Check if OpenSearch is available
if ! curl -s "$OPENSEARCH_ENDPOINT/_cluster/health" > /dev/null 2>&1; then
    echo "SKIP: OpenSearch not available at $OPENSEARCH_ENDPOINT"
    exit 0
fi

# Delete index if exists
curl -s -X DELETE "$OPENSEARCH_ENDPOINT/$INDEX" > /dev/null 2>&1 || true

# Create gzipped log file
TEMP_LOG=$(mktemp)
TEMP_GZ="${TEMP_LOG}.gz"
echo "$LOG_ENTRY" > "$TEMP_LOG"
gzip -c "$TEMP_LOG" > "$TEMP_GZ"

# Setup S3
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 mb "s3://$BUCKET" 2>/dev/null || true
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 cp "$TEMP_GZ" "s3://$BUCKET/logs/test.log.gz"

# Run forwarder with OpenSearch output
export AWS_ENDPOINT_URL="$LOCALSTACK_ENDPOINT"
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_REGION="eu-west-1"
export OUTPUTS="opensearch"
export ELASTICSEARCH_ENDPOINT="$OPENSEARCH_ENDPOINT"
export ELASTICSEARCH_INDEX="$INDEX"

"$BINARY" "s3://$BUCKET/logs/"

# Wait for indexing
sleep 2

# Refresh index and verify document
curl -s -X POST "$OPENSEARCH_ENDPOINT/$INDEX/_refresh" > /dev/null

DOC_COUNT=$(curl -s "$OPENSEARCH_ENDPOINT/$INDEX/_count" | grep -o '"count":[0-9]*' | grep -o '[0-9]*')

if [ "$DOC_COUNT" -lt 1 ]; then
    echo "ERROR: No documents found in OpenSearch index"
    exit 1
fi

echo "OpenSearch output verified: $DOC_COUNT documents indexed"

# Cleanup
rm -f "$TEMP_LOG" "$TEMP_GZ"
curl -s -X DELETE "$OPENSEARCH_ENDPOINT/$INDEX" > /dev/null 2>&1 || true
aws --endpoint-url="$LOCALSTACK_ENDPOINT" s3 rb "s3://$BUCKET" --force 2>/dev/null || true
