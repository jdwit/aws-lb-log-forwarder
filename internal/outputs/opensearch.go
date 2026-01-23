package outputs

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jdwit/aws-lb-log-forwarder/internal/types"
)

const (
	osMaxBatchSize = 5_000_000 // 5MB
	osMaxEvents    = 500
)

// OpenSearch sends log entries to OpenSearch.
type OpenSearch struct {
	client   *http.Client
	endpoint string
	index    string
	username string
	password string
}

// NewOpenSearch creates an OpenSearch output from environment configuration.
func NewOpenSearch() (*OpenSearch, error) {
	endpoint, err := requiredEnv("OPENSEARCH_ENDPOINT")
	if err != nil {
		return nil, err
	}

	index, err := requiredEnv("OPENSEARCH_INDEX")
	if err != nil {
		return nil, err
	}

	// Allow skipping TLS verification for self-signed certs
	skipVerify := os.Getenv("OPENSEARCH_SKIP_VERIFY") == "true"

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
		},
	}

	return &OpenSearch{
		client:   client,
		endpoint: endpoint,
		index:    index,
		username: os.Getenv("OPENSEARCH_USERNAME"),
		password: os.Getenv("OPENSEARCH_PASSWORD"),
	}, nil
}

// SendLogs receives entries and batches them to OpenSearch using the bulk API.
func (o *OpenSearch) SendLogs(ctx context.Context, entries <-chan types.LogEntry) {
	var batch []types.LogEntry
	var batchSize int

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		o.send(ctx, batch)
		batch = nil
		batchSize = 0
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return

		case entry, ok := <-entries:
			if !ok {
				flush()
				return
			}

			data, _ := json.Marshal(entry.Data)
			eventSize := len(data)

			if len(batch) > 0 && (batchSize+eventSize > osMaxBatchSize || len(batch) >= osMaxEvents) {
				flush()
			}

			batch = append(batch, entry)
			batchSize += eventSize

		case <-ticker.C:
			flush()
		}
	}
}

func (o *OpenSearch) send(ctx context.Context, entries []types.LogEntry) {
	// Build bulk request body (NDJSON format)
	var buf bytes.Buffer
	for _, entry := range entries {
		// Action line
		action := map[string]any{
			"index": map[string]any{
				"_index": o.index,
			},
		}
		actionData, _ := json.Marshal(action)
		buf.Write(actionData)
		buf.WriteByte('\n')

		// Document line (include timestamp as @timestamp for OpenSearch Dashboards compatibility)
		doc := make(map[string]any, len(entry.Data)+1)
		for k, v := range entry.Data {
			doc[k] = v
		}
		doc["@timestamp"] = entry.Timestamp.Format(time.RFC3339Nano)

		docData, _ := json.Marshal(doc)
		buf.Write(docData)
		buf.WriteByte('\n')
	}

	url := fmt.Sprintf("%s/_bulk", o.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		slog.Error("opensearch request failed", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/x-ndjson")

	if o.username != "" && o.password != "" {
		req.SetBasicAuth(o.username, o.password)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		slog.Error("opensearch send failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Error("opensearch error", "status", resp.StatusCode)
	}
}
