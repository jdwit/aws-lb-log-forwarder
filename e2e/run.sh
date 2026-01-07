#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

FAILURES=""

echo "Building binary..."
cd "$PROJECT_DIR"
go build -o "$SCRIPT_DIR/aws-lb-log-forwarder" .

echo ""
echo "Running e2e tests..."
echo ""

for test_file in "$SCRIPT_DIR"/test_*.sh; do
    test_name=$(basename "$test_file")
    echo "Running $test_name..."

    if bash "$test_file"; then
        echo -e "${GREEN}PASS${NC} $test_name"
    else
        echo -e "${RED}FAIL${NC} $test_name"
        FAILURES="$FAILURES\n  - $test_name"
    fi
    echo ""
done

# Cleanup
rm -f "$SCRIPT_DIR/aws-lb-log-forwarder"

if [ -n "$FAILURES" ]; then
    echo -e "${RED}Failed tests:${NC}"
    echo -e "$FAILURES"
    exit 1
fi

echo -e "${GREEN}All e2e tests passed!${NC}"
