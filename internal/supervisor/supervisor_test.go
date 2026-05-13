package supervisor

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestProxyHandlerSwitchesCachedWorkerTarget(t *testing.T) {
	backendOne := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "one")
	}))
	defer backendOne.Close()
	backendTwo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "two")
	}))
	defer backendTwo.Close()

	var target atomic.Value
	target.Store(backendOne.URL)
	handler := proxyHandler(&target)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/value", nil))
	if rec.Body.String() != "one" {
		t.Fatalf("expected first backend response, got %q", rec.Body.String())
	}

	target.Store(backendTwo.URL)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/value", nil))
	if rec.Body.String() != "two" {
		t.Fatalf("expected second backend response after target switch, got %q", rec.Body.String())
	}
}

func BenchmarkProxyHandlerCachedTarget(b *testing.B) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	var target atomic.Value
	target.Store(backend.URL)
	handler := proxyHandler(&target)
	req := httptest.NewRequest(http.MethodGet, "/value", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
