package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/jdwit/aws-lb-log-forwarder/internal/logprocessor"
)

func main() {
	// LoadDefaultConfig honours AWS_ENDPOINT_URL for local testing (e.g. LocalStack).
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		slog.Error("aws config failed", "error", err)
		os.Exit(1)
	}

	proc, err := logprocessor.New(cfg)
	if err != nil {
		slog.Error("processor init failed", "error", err)
		os.Exit(1)
	}

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		slog.Info("starting lambda handler")
		lambda.Start(proc.HandleLambdaEvent)
		return
	}

	if len(os.Args) < 2 {
		slog.Error("usage: alb-log-forwarder <s3-url>")
		os.Exit(1)
	}

	slog.Info("processing S3 URL", "url", os.Args[1])
	if err := proc.HandleS3URL(context.Background(), os.Args[1]); err != nil {
		slog.Error("processing failed", "error", err)
		os.Exit(1)
	}
}
