# ALB Log Forwarder

Forward AWS ALB access logs from S3 to various outputs.

## How It Works

ALB writes gzipped access logs to S3. This tool runs as a Lambda function triggered by S3 events—each time a new log file lands, Lambda processes it and forwards the entries to your configured outputs.

```
ALB → S3 bucket → S3 event → Lambda → outputs
```

## Deployment

See [terraform-aws-alb-log-forwarder](https://github.com/jdwit/terraform-aws-alb-log-forwarder) for the Terraform module. Includes Lambda deployment, S3 trigger, and CloudWatch alarm on failures.

## Supported Outputs

- `cloudwatch` – CloudWatch Logs
- `firehose` – Kinesis Data Firehose
- `splunk` – Splunk HEC
- `stdout` – Write to stdout for testing

## Configuration

| Variable | Description |
|----------|-------------|
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
go install github.com/jdwit/alb-log-forwarder@latest
OUTPUTS=stdout alb-log-forwarder s3://bucket/path/to/logs/
```
