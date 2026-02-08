package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExecutorTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exec := NewExecutor(WithTimeout(20 * time.Millisecond))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	resp, err := exec.Do(context.Background(), req)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if resp.Duration <= 0 {
		t.Fatalf("expected duration to be set")
	}
}
