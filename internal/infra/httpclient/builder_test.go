package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestBuildRequestJSON(t *testing.T) {
	payload := map[string]any{"foo": "bar"}
	assert := func(r *http.Request, body []byte) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected method POST, got %s", r.Method)
		}
		if r.URL.Path != "/json" {
			t.Fatalf("expected path /json, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Test") != "yes" {
			t.Fatalf("expected header X-Test")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected content-type json, got %s", ct)
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("expected valid json body: %v", err)
		}
		if decoded["foo"] != "bar" {
			t.Fatalf("expected json payload")
		}
	}

	runRequest(t, domain.RequestSpec{
		Method:  domain.MethodPost,
		URL:     "",
		Headers: domain.Headers{"X-Test": "yes"},
		Body: domain.BodySpec{
			Type: domain.BodyJSON,
			JSON: payload,
		},
	}, "/json", assert)
}

func TestBuildRequestForm(t *testing.T) {
	assert := func(r *http.Request, body []byte) {
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Fatalf("expected form content-type, got %s", ct)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("expected form body: %v", err)
		}
		if values.Get("token") != "abc" {
			t.Fatalf("expected token value")
		}
	}

	runRequest(t, domain.RequestSpec{
		Method: domain.MethodPost,
		URL:    "",
		Body: domain.BodySpec{
			Type: domain.BodyForm,
			Form: map[string]string{"token": "abc"},
		},
	}, "/form", assert)
}

func TestBuildRequestRaw(t *testing.T) {
	assert := func(r *http.Request, body []byte) {
		if ct := r.Header.Get("Content-Type"); ct != "text/plain" {
			t.Fatalf("expected raw content-type, got %s", ct)
		}
		if strings.TrimSpace(string(body)) != "raw-body" {
			t.Fatalf("expected raw body")
		}
	}

	runRequest(t, domain.RequestSpec{
		Method: domain.MethodPut,
		URL:    "",
		Body: domain.BodySpec{
			Type:        domain.BodyRaw,
			Raw:         "raw-body",
			ContentType: "text/plain",
		},
	}, "/raw", assert)
}

func runRequest(t *testing.T, spec domain.RequestSpec, path string, assert func(*http.Request, []byte)) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading body: %v", err)
		}
		assert(r, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	spec.URL = server.URL + path

	req, err := BuildRequest(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed request: %v", err)
	}
	resp.Body.Close()
}
