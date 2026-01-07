package outputs

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jdwit/alb-log-forwarder/internal/types"
)

const (
	splunkMaxBatchSize = 1_000_000 // 1MB
	splunkMaxEvents    = 100
)

// Splunk sends log entries to Splunk HEC (HTTP Event Collector).
type Splunk struct {
	client     *http.Client
	endpoint   string
	token      string
	source     string
	sourcetype string
	index      string
}

type splunkEvent struct {
	Time       int64             `json:"time"`
	Source     string            `json:"source,omitempty"`
	Sourcetype string            `json:"sourcetype,omitempty"`
	Index      string            `json:"index,omitempty"`
	Event      map[string]string `json:"event"`
}

// NewSplunk creates a Splunk HEC output from environment configuration.
func NewSplunk() (*Splunk, error) {
	endpoint, err := requiredEnv("SPLUNK_HEC_ENDPOINT")
	if err != nil {
		return nil, err
	}

	token, err := requiredEnv("SPLUNK_HEC_TOKEN")
	if err != nil {
		return nil, err
	}

	// Allow skipping TLS verification for self-signed certs
	skipVerify := os.Getenv("SPLUNK_SKIP_VERIFY") == "true"

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
		},
	}

	return &Splunk{
		client:     client,
		endpoint:   endpoint,
		token:      token,
		source:     os.Getenv("SPLUNK_SOURCE"),
		sourcetype: os.Getenv("SPLUNK_SOURCETYPE"),
		index:      os.Getenv("SPLUNK_INDEX"),
	}, nil
}

// SendLogs receives entries and batches them to Splunk HEC.
func (s *Splunk) SendLogs(ctx context.Context, entries <-chan types.LogEntry) {
	var batch []splunkEvent
	var batchSize int

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		s.send(ctx, batch)
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

			event := splunkEvent{
				Time:       entry.Timestamp.Unix(),
				Source:     s.source,
				Sourcetype: s.sourcetype,
				Index:      s.index,
				Event:      entry.Data,
			}

			data, _ := json.Marshal(event)
			eventSize := len(data)

			if len(batch) > 0 && (batchSize+eventSize > splunkMaxBatchSize || len(batch) >= splunkMaxEvents) {
				flush()
			}

			batch = append(batch, event)
			batchSize += eventSize

		case <-ticker.C:
			flush()
		}
	}
}

func (s *Splunk) send(ctx context.Context, events []splunkEvent) {
	var buf bytes.Buffer
	for _, e := range events {
		data, _ := json.Marshal(e)
		buf.Write(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, &buf)
	if err != nil {
		slog.Error("splunk request failed", "error", err)
		return
	}

	req.Header.Set("Authorization", "Splunk "+s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Error("splunk send failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("splunk error", "status", resp.StatusCode)
	}
}
