# AWS Load Balancer Log Forwarder

Forward AWS ALB and NLB access logs from S3 to various outputs.

## How It Works

AWS load balancers write gzipped access logs to S3. This tool runs as a Lambda function triggered by S3 events—each time a new log file lands, Lambda processes it and forwards the entries to your configured outputs.

```
ALB/NLB → S3 bucket → S3 event → Lambda → outputs
```

## Deployment

See [terraform-aws-lb-log-forwarder](https://github.com/jdwit/terraform-aws-lb-log-forwarder) for the Terraform module. Includes Lambda deployment, S3 trigger, and CloudWatch alarm on failures.

## Supported Load Balancers

- **ALB** (Application Load Balancer) — HTTP/HTTPS access logs
- **NLB** (Network Load Balancer) — TLS access logs

Field definitions from AWS docs:
- [ALB access log fields](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html)
- [NLB access log fields](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-access-logs.html)

## Supported Outputs

- `cloudwatch` – CloudWatch Logs
- `firehose` – Kinesis Data Firehose
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
| `FIREHOSE_STREAM_NAME` | Kinesis Firehose delivery stream |
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
