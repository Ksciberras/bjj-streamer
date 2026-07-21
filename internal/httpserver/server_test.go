package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPinger struct{ err error }

func (p stubPinger) Ping(context.Context) error { return p.err }

func TestHealthIsLiveWithoutDatabase(t *testing.T) {
	response := httptest.NewRecorder()
	New(discardLogger(), stubPinger{err: errors.New("down")}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK || response.Header().Get(requestIDHeader) == "" {
		t.Fatalf("status=%d request-id=%q", response.Code, response.Header().Get(requestIDHeader))
	}
}

func TestReadinessReflectsDatabase(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{{"ready", nil, http.StatusOK}, {"unavailable", errors.New("down"), http.StatusServiceUnavailable}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			New(discardLogger(), stubPinger{err: test.err}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if response.Code != test.want {
				t.Fatalf("got %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestRequestIDIsPreserved(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set(requestIDHeader, "upstream-id")
	response := httptest.NewRecorder()
	New(discardLogger(), stubPinger{}).ServeHTTP(response, request)
	if got := response.Header().Get(requestIDHeader); got != "upstream-id" {
		t.Fatalf("got %q", got)
	}
}

func discardLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }
