package proxy_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"reverse-proxy/proxy"
)

// TestRoundRobinDistribution verifies that requests are evenly distributed
// in round-robin fashion across all healthy backend servers.
func TestRoundRobinDistribution(t *testing.T) {
	var count1, count2, count3 atomic.Int32

	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count1.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s1"))
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count2.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s2"))
	}))
	defer s2.Close()

	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count3.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s3"))
	}))
	defer s3.Close()

	urls := []string{s1.URL, s2.URL, s3.URL}
	b := proxy.NewBalancer(urls)

	// Send 30 requests sequentially
	const totalRequests = 30
	for range totalRequests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		b.Handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rec.Code)
		}
	}

	expectedPerServer := int32(totalRequests / 3)
	if count1.Load() != expectedPerServer || count2.Load() != expectedPerServer || count3.Load() != expectedPerServer {
		t.Errorf("Round Robin failed: s1=%d, s2=%d, s3=%d (expected %d each)",
			count1.Load(), count2.Load(), count3.Load(), expectedPerServer)
	}
}

// TestFailoverAndRetry verifies that when a backend fails/closes,
// the proxy automatically marks it inactive and transparently retries on a healthy server.
func TestFailoverAndRetry(t *testing.T) {
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s1"))
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s2"))
	}))
	// We close s2 immediately to simulate an outage
	s2.Close()

	b := proxy.NewBalancer([]string{s1.URL, s2.URL})

	// First request might hit s2, fail, disable s2, and retry on s1
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	b.Handler(rec, req)

	// Even though s2 was down, the client should get a 200 OK from the retry on s1!
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected failover retry to succeed with 200 OK, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	if string(body) != "s1" {
		t.Errorf("Expected response body 's1', got %s", string(body))
	}
}

// TestHealthCheckRecovery verifies that an inactive backend is automatically
// re-enabled when the health check confirms it is back online.
func TestHealthCheckRecovery(t *testing.T) {
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s1.Close()

	b := proxy.NewBalancer([]string{s1.URL})

	// Manually disable the server
	b.Proxies[0].IsActive.Store(false)
	if b.Proxies[0].IsActive.Load() {
		t.Fatal("Expected proxy to be inactive")
	}

	// Run HealthCheck
	b.HealthCheck()

	// Should be revived
	if !b.Proxies[0].IsActive.Load() {
		t.Errorf("Expected health check to reactivate the server, but it remained inactive")
	}
}

// TestAllBackendsDown verifies that if all backends are unavailable,
// the proxy gracefully responds with 503 Service Unavailable.
func TestAllBackendsDown(t *testing.T) {
	b := proxy.NewBalancer([]string{"http://127.0.0.1:59999"})
	// Mark all inactive
	b.Proxies[0].IsActive.Store(false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	b.Handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503 Service Unavailable, got %d", rec.Code)
	}
}

// TestConcurrentRequests verifies race-condition safety under high concurrency.
func TestConcurrentRequests(t *testing.T) {
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s2.Close()

	b := proxy.NewBalancer([]string{s1.URL, s2.URL})

	ctx := t.Context()

	// Background health checks running concurrently with requests
	go b.StartHealthCheck(ctx, 20*time.Millisecond)

	var wg sync.WaitGroup
	const totalRoutines = 100

	for i := range totalRoutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?q=%d", idx), nil)
			rec := httptest.NewRecorder()
			b.Handler(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Unexpected status code under concurrency: %d", rec.Code)
			}
		}(i)
	}

	wg.Wait()
}
