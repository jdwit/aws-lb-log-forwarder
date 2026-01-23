# AWS Load Balancer Log Forwarder

[![CI](https://github.com/jdwit/aws-lb-log-forwarder/actions/workflows/ci.yml/badge.svg)](https://github.com/jdwit/aws-lb-log-forwarder/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jdwit/aws-lb-log-forwarder)](https://goreportcard.com/report/github.com/jdwit/aws-lb-log-forwarder)

Forward AWS ALB and NLB access logs from S3 to various outputs.

## How It Works

AWS load balancers write gzipped access logs to S3. This tool runs as a Lambda function triggered by S3 events; each time a new log file lands, Lambda processes it and forwards the entries to your configured outputs.

```
ALB/NLB → S3 bucket → S3 event → Lambda → outputs
```

## Deployment

See [terraform-aws-lb-log-forwarder](https://github.com/jdwit/terraform-aws-lb-log-forwarder) for the Terraform module. Includes Lambda deployment, S3 trigger, and CloudWatch alarm on failures.

### TODO

- [ ] Create ECR Public repository `aws-lb-log-forwarder` under alias `jdwit` in us-east-1
- [ ] Create IAM role with OIDC trust for GitHub Actions
- [ ] Add `AWS_ROLE_ARN` secret to this repository

Field definitions from AWS docs:
- [ALB access log fields](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html)
- [NLB access log fields](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-access-logs.html)

## Supported Outputs

- `cloudwatch` – CloudWatch Logs
- `opensearch` – OpenSearch
- `splunk` – Splunk HEC
- `stdout` – Write to stdout for testing

## Configuration

| Variable | Description |
|----------|-------------|
| `LB_TYPE` | Load balancer type: `alb` (default) or `nlb` |
| `OUTPUTS` | Required. Comma-separated list of outputs |
| `FIELDS` | Optional. Comma-separated fields to include (default: all) |
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
OUTPUTS=stdout aws-lb-log-forwarder s3://bucket/path/to/alb-logs/

# NLB logs
LB_TYPE=nlb OUTPUTS=stdout aws-lb-log-forwarder s3://bucket/path/to/nlb-logs/
```
