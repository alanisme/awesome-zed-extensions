package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"golang.org/x/time/rate"
)

func newTestClient(url string) *Client {
	return &Client{
		http:    &http.Client{},
		token:   "test-token",
		limiter: rate.NewLimiter(rate.Inf, 1),
		retries: 1,
	}
}

func TestGetRepoInfo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stargazers_count":42,"description":"test repo","default_branch":"main"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	// Override to hit test server.
	info, etag, err := c.getRepoInfoURL(context.Background(), srv.URL+"/repos/owner/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.Stars != 42 {
		t.Errorf("stars = %d, want 42", info.Stars)
	}
	if etag != `"abc123"` {
		t.Errorf("etag = %q, want %q", etag, `"abc123"`)
	}
}

func TestGetRepoInfo_304NotModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"old-etag"` {
			w.Header().Set("ETag", `"old-etag"`)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		t.Error("expected If-None-Match header")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	info, etag, err := c.getRepoInfoURL(context.Background(), srv.URL+"/repos/owner/repo", `"old-etag"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil info for 304")
	}
	if etag != `"old-etag"` {
		t.Errorf("etag = %q, want %q", etag, `"old-etag"`)
	}
}

func TestGetRepoInfo_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	info, _, err := c.getRepoInfoURL(context.Background(), srv.URL+"/repos/owner/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil info for 404")
	}
}

func TestGetRepoInfo_ETagUpdatedOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server returns new ETag even when old one was sent.
		w.Header().Set("ETag", `"new-etag"`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stargazers_count":10,"default_branch":"main"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	info, etag, err := c.getRepoInfoURL(context.Background(), srv.URL+"/repos/owner/repo", `"old-etag"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected info")
	}
	if etag != `"new-etag"` {
		t.Errorf("etag = %q, want %q", etag, `"new-etag"`)
	}
}

func TestGetRepoInfo_RetryOnServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"stargazers_count":5,"default_branch":"main"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	info, _, err := c.getRepoInfoURL(context.Background(), srv.URL+"/repos/owner/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected info after retry")
	}
	if info.Stars != 5 {
		t.Errorf("stars = %d, want 5", info.Stars)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestGetRawFile_304NotModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"file-etag"` {
			w.Header().Set("ETag", `"file-etag"`)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		t.Error("expected If-None-Match header")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	body, etag, err := c.getRawFileURL(context.Background(), srv.URL+"/file", `"file-etag"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != nil {
		t.Error("expected nil body for 304")
	}
	if etag != `"file-etag"` {
		t.Errorf("etag = %q, want %q", etag, `"file-etag"`)
	}
}

func TestGetRepoInfo_InvalidName(t *testing.T) {
	c := newTestClient("")
	_, _, err := c.GetRepoInfo(context.Background(), "bad/name", "repo", "")
	if err == nil {
		t.Error("expected error for invalid owner name")
	}
}
