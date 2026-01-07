package outputs

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jdwit/aws-lb-log-forwarder/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestElasticsearch_Send(t *testing.T) {
	t.Run("Send documents successfully", func(t *testing.T) {
		var receivedDocs []map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/_bulk", r.URL.Path)
			assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))

			scanner := bufio.NewScanner(r.Body)
			for scanner.Scan() {
				line := scanner.Text()
				var doc map[string]any
				if err := json.Unmarshal([]byte(line), &doc); err == nil {
					// Skip action lines, only capture document lines
					if _, hasIndex := doc["index"]; !hasIndex {
						receivedDocs = append(receivedDocs, doc)
					}
				}
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"errors":false}`))
		}))
		defer server.Close()

		es := &Elasticsearch{
			client:   server.Client(),
			endpoint: server.URL,
			index:    "test-index",
		}

		entries := []types.LogEntry{
			{Timestamp: time.Now(), Data: map[string]string{"message": "test1"}},
			{Timestamp: time.Now(), Data: map[string]string{"message": "test2"}},
		}

		es.send(context.Background(), entries)
		assert.Len(t, receivedDocs, 2)
		assert.Equal(t, "test1", receivedDocs[0]["message"])
		assert.Equal(t, "test2", receivedDocs[1]["message"])
	})

	t.Run("Basic auth header set", func(t *testing.T) {
		var authHeader string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		es := &Elasticsearch{
			client:   server.Client(),
			endpoint: server.URL,
			index:    "test-index",
			username: "elastic",
			password: "secret",
		}

		es.send(context.Background(), []types.LogEntry{
			{Timestamp: time.Now(), Data: map[string]string{"message": "test"}},
		})

		assert.Contains(t, authHeader, "Basic")
	})
}

func TestElasticsearch_SendLogs(t *testing.T) {
	t.Run("Process entries", func(t *testing.T) {
		docCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scanner := bufio.NewScanner(r.Body)
			for scanner.Scan() {
				line := scanner.Text()
				var doc map[string]any
				if err := json.Unmarshal([]byte(line), &doc); err == nil {
					if _, hasIndex := doc["index"]; !hasIndex {
						docCount++
					}
				}
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		es := &Elasticsearch{
			client:   server.Client(),
			endpoint: server.URL,
			index:    "test-index",
		}

		entries := make(chan types.LogEntry, 2)
		entries <- types.LogEntry{
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "test1"},
		}
		entries <- types.LogEntry{
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "test2"},
		}
		close(entries)

		es.SendLogs(context.Background(), entries)
		assert.Equal(t, 2, docCount)
	})
}

func TestNewElasticsearch(t *testing.T) {
	t.Run("Missing endpoint", func(t *testing.T) {
		t.Setenv("ELASTICSEARCH_ENDPOINT", "")
		t.Setenv("ELASTICSEARCH_INDEX", "logs")
		_, err := NewElasticsearch()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ELASTICSEARCH_ENDPOINT")
	})

	t.Run("Missing index", func(t *testing.T) {
		t.Setenv("ELASTICSEARCH_ENDPOINT", "https://localhost:9200")
		t.Setenv("ELASTICSEARCH_INDEX", "")
		_, err := NewElasticsearch()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ELASTICSEARCH_INDEX")
	})

	t.Run("Valid config", func(t *testing.T) {
		t.Setenv("ELASTICSEARCH_ENDPOINT", "https://localhost:9200")
		t.Setenv("ELASTICSEARCH_INDEX", "lb-logs")
		t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
		t.Setenv("ELASTICSEARCH_PASSWORD", "secret")

		es, err := NewElasticsearch()
		require.NoError(t, err)
		assert.Equal(t, "https://localhost:9200", es.endpoint)
		assert.Equal(t, "lb-logs", es.index)
		assert.Equal(t, "elastic", es.username)
		assert.Equal(t, "secret", es.password)
	})
}
