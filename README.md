# AWS Load Balancer Log Forwarder

[![CI](https://github.com/jdwit/aws-lb-log-forwarder/actions/workflows/ci.yml/badge.svg)](https://github.com/jdwit/aws-lb-log-forwarder/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jdwit/aws-lb-log-forwarder)](https://goreportcard.com/report/github.com/jdwit/aws-lb-log-forwarder)

Forward AWS ALB and NLB access logs from S3 to various destinations.

## How It Works

AWS load balancers write gzipped access logs to S3. This tool runs as a Lambda function triggered by S3 events; each time a new log file lands, Lambda processes it and forwards the entries to your configured destinations.

```
ALB/NLB → S3 bucket → S3 event → Lambda → destinations
```

### Streaming Architecture

Most log forwarding solutions load entire log files into memory before processing. This doesn't scale: during peak traffic, log files grow large, Lambda functions run out of memory, and you end up with gaps in your data.

This tool processes log files as a streaming pipeline with bounded memory usage. Each stage runs in its own goroutine, connected by channels with backpressure. The pipeline streams data without buffering entire files in memory.

## Deployment

See [terraform-aws-lb-log-forwarder](https://github.com/jdwit/terraform-aws-lb-log-forwarder) for the Terraform module.

Field definitions from AWS docs:
- [ALB access log fields](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html)
- [NLB access log fields](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-access-logs.html)

## Supported Destinations

- `cloudwatch` – CloudWatch Logs
- `opensearch` – OpenSearch
- `splunk` – Splunk HEC
- `stdout` – Write to stdout for testing

## Configuration

| Variable | Description |
|----------|-------------|
| `LB_TYPE` | Load balancer type: `alb` (default) or `nlb` |
| `DESTINATIONS` | Required. Comma-separated list of destinations |
| `FIELDS` | Optional. Comma-separated fields to include (default: all) |
| `BUFFER_SIZE` | Optional. Channel buffer size in number of log entries (default: 2000) |
| `CLOUDWATCH_LOG_GROUP` | CloudWatch log group name |
| `CLOUDWATCH_LOG_STREAM` | CloudWatch log stream name |
| `OPENSEARCH_ENDPOINT` | OpenSearch URL (e.g., `https://localhost:9200`) |
| `OPENSEARCH_INDEX` | Index name for documents |
| `OPENSEARCH_USERNAME` | Optional. Basic auth username |
| `OPENSEARCH_PASSWORD` | Optional. Basic auth password |
| `OPENSEARCH_SKIP_VERIFY` | Optional. Set to `true` to skip TLS verification |
| `SPLUNK_HEC_ENDPOINT` | Splunk HEC URL |
| `SPLUNK_HEC_TOKEN` | Splunk HEC token |
| `SPLUNK_SOURCE` | Optional. Splunk source field |
| `SPLUNK_SOURCETYPE` | Optional. Splunk sourcetype field |
| `SPLUNK_INDEX` | Optional. Splunk index |

## CLI Usage

Can also run standalone for testing or backfilling:

```bash
go install github.com/jdwit/aws-lb-log-forwarder@latest

# ALB logs (default)
DESTINATIONS=stdout aws-lb-log-forwarder s3://bucket/path/to/alb-logs/

# NLB logs
LB_TYPE=nlb DESTINATIONS=stdout aws-lb-log-forwarder s3://bucket/path/to/nlb-logs/
```
