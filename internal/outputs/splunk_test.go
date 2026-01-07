package outputs

import (
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

func TestSplunk_Send(t *testing.T) {
	t.Run("Send events successfully", func(t *testing.T) {
		var receivedEvents []splunkEvent

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Splunk test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			decoder := json.NewDecoder(r.Body)
			for decoder.More() {
				var event splunkEvent
				if err := decoder.Decode(&event); err == nil {
					receivedEvents = append(receivedEvents, event)
				}
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		splunk := &Splunk{
			client:   server.Client(),
			endpoint: server.URL,
			token:    "test-token",
		}

		events := []splunkEvent{
			{Time: 1234567890, Event: map[string]string{"message": "test1"}},
			{Time: 1234567891, Event: map[string]string{"message": "test2"}},
		}

		splunk.send(context.Background(), events)
		assert.Len(t, receivedEvents, 2)
	})
}

func TestSplunk_SendLogs(t *testing.T) {
	t.Run("Process entries", func(t *testing.T) {
		eventCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			decoder := json.NewDecoder(r.Body)
			for decoder.More() {
				var event splunkEvent
				if err := decoder.Decode(&event); err == nil {
					eventCount++
				}
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		splunk := &Splunk{
			client:   server.Client(),
			endpoint: server.URL,
			token:    "test-token",
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

		splunk.SendLogs(context.Background(), entries)
		assert.Equal(t, 2, eventCount)
	})
}

func TestNewSplunk(t *testing.T) {
	t.Run("Missing endpoint", func(t *testing.T) {
		t.Setenv("SPLUNK_HEC_ENDPOINT", "")
		t.Setenv("SPLUNK_HEC_TOKEN", "token")
		_, err := NewSplunk()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SPLUNK_HEC_ENDPOINT")
	})

	t.Run("Missing token", func(t *testing.T) {
		t.Setenv("SPLUNK_HEC_ENDPOINT", "https://example.com")
		t.Setenv("SPLUNK_HEC_TOKEN", "")
		_, err := NewSplunk()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SPLUNK_HEC_TOKEN")
	})

	t.Run("Valid config", func(t *testing.T) {
		t.Setenv("SPLUNK_HEC_ENDPOINT", "https://example.com")
		t.Setenv("SPLUNK_HEC_TOKEN", "test-token")
		t.Setenv("SPLUNK_SOURCE", "alb")
		t.Setenv("SPLUNK_SOURCETYPE", "aws:alb")
		t.Setenv("SPLUNK_INDEX", "main")

		splunk, err := NewSplunk()
		require.NoError(t, err)
		assert.Equal(t, "https://example.com", splunk.endpoint)
		assert.Equal(t, "test-token", splunk.token)
		assert.Equal(t, "alb", splunk.source)
		assert.Equal(t, "aws:alb", splunk.sourcetype)
		assert.Equal(t, "main", splunk.index)
	})
}
