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
	esMaxBatchSize = 5_000_000 // 5MB
	esMaxEvents    = 500
)

// Elasticsearch sends log entries to Elasticsearch or OpenSearch.
type Elasticsearch struct {
	client   *http.Client
	endpoint string
	index    string
	username string
	password string
}

// NewElasticsearch creates an Elasticsearch/OpenSearch output from environment configuration.
func NewElasticsearch() (*Elasticsearch, error) {
	endpoint, err := requiredEnv("ELASTICSEARCH_ENDPOINT")
	if err != nil {
		return nil, err
	}

	index, err := requiredEnv("ELASTICSEARCH_INDEX")
	if err != nil {
		return nil, err
	}

	// Allow skipping TLS verification for self-signed certs
	skipVerify := os.Getenv("ELASTICSEARCH_SKIP_VERIFY") == "true"

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
		},
	}

	return &Elasticsearch{
		client:   client,
		endpoint: endpoint,
		index:    index,
		username: os.Getenv("ELASTICSEARCH_USERNAME"),
		password: os.Getenv("ELASTICSEARCH_PASSWORD"),
	}, nil
}

// SendLogs receives entries and batches them to Elasticsearch using the bulk API.
func (e *Elasticsearch) SendLogs(ctx context.Context, entries <-chan types.LogEntry) {
	var batch []types.LogEntry
	var batchSize int

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		e.send(ctx, batch)
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

			if len(batch) > 0 && (batchSize+eventSize > esMaxBatchSize || len(batch) >= esMaxEvents) {
				flush()
			}

			batch = append(batch, entry)
			batchSize += eventSize

		case <-ticker.C:
			flush()
		}
	}
}

func (e *Elasticsearch) send(ctx context.Context, entries []types.LogEntry) {
	// Build bulk request body (NDJSON format)
	var buf bytes.Buffer
	for _, entry := range entries {
		// Action line
		action := map[string]any{
			"index": map[string]any{
				"_index": e.index,
			},
		}
		actionData, _ := json.Marshal(action)
		buf.Write(actionData)
		buf.WriteByte('\n')

		// Document line (include timestamp as @timestamp for Kibana compatibility)
		doc := make(map[string]any, len(entry.Data)+1)
		for k, v := range entry.Data {
			doc[k] = v
		}
		doc["@timestamp"] = entry.Timestamp.Format(time.RFC3339Nano)

		docData, _ := json.Marshal(doc)
		buf.Write(docData)
		buf.WriteByte('\n')
	}

	url := fmt.Sprintf("%s/_bulk", e.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		slog.Error("elasticsearch request failed", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/x-ndjson")

	if e.username != "" && e.password != "" {
		req.SetBasicAuth(e.username, e.password)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		slog.Error("elasticsearch send failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Error("elasticsearch error", "status", resp.StatusCode)
	}
}
