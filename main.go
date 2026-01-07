package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jdwit/aws-lb-log-forwarder/internal/logprocessor"
)

func main() {
	sess, err := newSession()
	if err != nil {
		slog.Error("session failed", "error", err)
		os.Exit(1)
	}

	proc, err := logprocessor.New(sess)
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

func newSession() (*session.Session, error) {
	if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
		return session.NewSession(&aws.Config{
			Endpoint:         aws.String(endpoint),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
		})
	}
	return session.NewSession()
}
