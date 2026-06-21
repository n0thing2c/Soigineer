package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type fakeIngestor struct {
	singleErr error
	batchErr  error
	singles   []domain.LogRecord
	batches   [][]domain.LogRecord
}

func (f *fakeIngestor) IngestSingleLog(ctx context.Context, log domain.LogRecord) error {
	f.singles = append(f.singles, log)
	return f.singleErr
}

func (f *fakeIngestor) IngestBatchLog(ctx context.Context, logs []domain.LogRecord) error {
	f.batches = append(f.batches, logs)
	return f.batchErr
}

func newTestRouter(ingestor *fakeIngestor) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/v1"), NewLogHandler(ingestor))
	return router
}

func validDeliveryLog() LogRequest {
	return LogRequest{
		ApplicationName: "app-001",
		Level:           "INFO",
		Message:         "hello",
		Timestamp:       "2026-06-21T10:00:00Z",
		TraceID:         "trace-1",
	}
}

func performJSON(router http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	var body bytes.Buffer
	switch value := payload.(type) {
	case string:
		body.WriteString(value)
	default:
		_ = json.NewEncoder(&body).Encode(payload)
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestSingleLogHandleAccepted(t *testing.T) {
	ingestor := &fakeIngestor{}
	router := newTestRouter(ingestor)

	rec := performJSON(router, http.MethodPost, "/v1/ingest/logs", validDeliveryLog())
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(ingestor.singles) != 1 || ingestor.singles[0].TraceID != "trace-1" {
		t.Fatalf("singles = %#v", ingestor.singles)
	}
	if !strings.Contains(rec.Body.String(), `"traceId":"trace-1"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestBatchLogHandleAccepted(t *testing.T) {
	ingestor := &fakeIngestor{}
	router := newTestRouter(ingestor)

	rec := performJSON(router, http.MethodPost, "/v1/ingest/logs/batch", BatchLogRequest{
		Logs: []LogRequest{validDeliveryLog(), validDeliveryLog()},
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(ingestor.batches) != 1 || len(ingestor.batches[0]) != 2 {
		t.Fatalf("batches = %#v", ingestor.batches)
	}
	if !strings.Contains(rec.Body.String(), `"count":2`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestSingleLogHandleValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		want    string
	}{
		{"invalid json", `{"applicationName":`, "Invalid JSON."},
		{"missing field", LogRequest{Level: "INFO", Message: "hello", Timestamp: "2026-06-21T10:00:00Z", TraceID: "trace-1"}, "must not empty"},
		{"invalid level", LogRequest{ApplicationName: "app-001", Level: "DEBUG", Message: "hello", Timestamp: "2026-06-21T10:00:00Z", TraceID: "trace-1"}, "Field 'Level' must in"},
		{"invalid timestamp", LogRequest{ApplicationName: "app-001", Level: "INFO", Message: "hello", Timestamp: "not-time", TraceID: "trace-1"}, "valid RFC3339"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := performJSON(newTestRouter(&fakeIngestor{}), http.MethodPost, "/v1/ingest/logs", tt.payload)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.want) {
				t.Fatalf("status/body = %d/%s, want 400 containing %q", rec.Code, rec.Body.String(), tt.want)
			}
		})
	}
}

func TestBatchLogHandleValidationErrors(t *testing.T) {
	tooMany := make([]LogRequest, 501)
	for idx := range tooMany {
		tooMany[idx] = validDeliveryLog()
	}

	tests := []struct {
		name    string
		payload any
		want    string
	}{
		{"invalid json", `{"logs":`, "One or more logs is invalid"},
		{"empty batch", BatchLogRequest{Logs: nil}, "One or more logs is invalid"},
		{"oversized batch", BatchLogRequest{Logs: tooMany}, "One or more logs is invalid"},
		{"invalid item timestamp", BatchLogRequest{Logs: []LogRequest{{ApplicationName: "app-001", Level: "INFO", Message: "hello", Timestamp: "bad", TraceID: "trace-1"}}}, "logs[0]:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := performJSON(newTestRouter(&fakeIngestor{}), http.MethodPost, "/v1/ingest/logs/batch", tt.payload)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.want) {
				t.Fatalf("status/body = %d/%s, want 400 containing %q", rec.Code, rec.Body.String(), tt.want)
			}
		})
	}
}

func TestHandlersReturnServiceUnavailableWhenIngestorFails(t *testing.T) {
	router := newTestRouter(&fakeIngestor{
		singleErr: errors.New("queue down"),
		batchErr:  errors.New("queue down"),
	})

	single := performJSON(router, http.MethodPost, "/v1/ingest/logs", validDeliveryLog())
	if single.Code != http.StatusServiceUnavailable || !strings.Contains(single.Body.String(), "QUEUE_UNAVAILABLE") {
		t.Fatalf("single status/body = %d/%s", single.Code, single.Body.String())
	}

	batch := performJSON(router, http.MethodPost, "/v1/ingest/logs/batch", BatchLogRequest{Logs: []LogRequest{validDeliveryLog()}})
	if batch.Code != http.StatusServiceUnavailable || !strings.Contains(batch.Body.String(), "QUEUE_UNAVAILABLE") {
		t.Fatalf("batch status/body = %d/%s", batch.Code, batch.Body.String())
	}
}

func TestRegisterRoutesExposesIngestEndpoints(t *testing.T) {
	router := newTestRouter(&fakeIngestor{})
	for _, path := range []string{"/v1/ingest/logs", "/v1/ingest/logs/batch"} {
		rec := performJSON(router, http.MethodPost, path, validDeliveryLog())
		if path == "/v1/ingest/logs/batch" {
			rec = performJSON(router, http.MethodPost, path, BatchLogRequest{Logs: []LogRequest{validDeliveryLog()}})
		}
		if rec.Code == http.StatusNotFound {
			t.Fatalf("%s returned 404", path)
		}
	}
}
