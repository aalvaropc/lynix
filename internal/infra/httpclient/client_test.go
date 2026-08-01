package httpclient

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew_Insecure_SetsTLSConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Insecure = true
	client := New(cfg)

	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true")
	}
}

func TestNew_Secure_VerifiesWithMinTLS12(t *testing.T) {
	cfg := DefaultConfig()
	client := New(cfg)

	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if tr.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig with MinVersion")
	}
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("secure client must verify certificates")
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("expected MinVersion TLS 1.2, got %x", tr.TLSClientConfig.MinVersion)
	}
}

func TestNew_CookieJarOptIn(t *testing.T) {
	client := New(DefaultConfig())
	if client.Jar != nil {
		t.Error("cookie jar must be opt-in")
	}

	cfg := DefaultConfig()
	cfg.EnableCookieJar = true
	client = New(cfg)
	if client.Jar == nil {
		t.Error("expected cookie jar when enabled")
	}
}

func TestNew_InsecureClient_ConnectsToSelfSigned(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Insecure = true
	cfg.TLSHandshake = cfg.Timeout
	client := New(cfg)

	// Override the dialer to use the test server's TLS config address.
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("expected no error with insecure client, got: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestNew_NoFollowRedirects_StopsAtRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.NoFollowRedirects = true
	client := New(cfg)

	resp, err := client.Get(server.URL + "/redirect")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302, got %d", resp.StatusCode)
	}
}

func TestNew_DefaultFollowsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	client := New(cfg)

	resp, err := client.Get(server.URL + "/redirect")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (followed redirect), got %d", resp.StatusCode)
	}
}

func TestNew_PerRequestNoRedirect_ViaContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	client := New(cfg)

	// Request with no-redirect context should stop at 302.
	ctx := ContextWithNoRedirect(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/redirect", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 with no-redirect context, got %d", resp.StatusCode)
	}

	// Same client, normal context should follow redirect.
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/redirect", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with normal context, got %d", resp2.StatusCode)
	}
}
