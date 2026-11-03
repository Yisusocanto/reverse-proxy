package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	url2 "net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Proxy struct {
	Url          *url2.URL
	IsActive     atomic.Bool
	ReverseProxy *httputil.ReverseProxy
}

type Balancer struct {
	Mu      sync.Mutex
	Counter int
	Proxies []*Proxy
}

func NewBalancer(urls []string) *Balancer {
	balancer := &Balancer{
		Proxies: make([]*Proxy, 0, len(urls)),
	}

	for _, u := range urls {
		url, err := url2.Parse(u)
		if err != nil {
			log.Fatalf("Error parsing the url %v", err)
		}
		serverProxy := &Proxy{
			Url: url,
		}
		serverProxy.IsActive.Store(true)

		serverProxy.ReverseProxy = &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(url)
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("Disabling proxy %s due to error: %v", u, err.Error())
				balancer.DisableProxy(serverProxy)
				balancer.Handler(w, r) // Retry the request with another server
			},
		}

		balancer.Proxies = append(balancer.Proxies, serverProxy)
	}
	return balancer
}

func (b *Balancer) Handler(w http.ResponseWriter, r *http.Request) {
	p, err := b.GetNextProxy()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte(`{"error": "Service unavailable"}`)); err != nil {
			log.Printf("Error responding to user: %v", err)
		}
		return
	}

	p.ServeHTTP(w, r)
}

func (b *Balancer) GetNextProxy() (*httputil.ReverseProxy, error) {
	b.Mu.Lock()
	defer b.Mu.Unlock()

	for i := 0; i < len(b.Proxies); i++ {
		p := b.Proxies[b.Counter%len(b.Proxies)]
		b.Counter++
		if p.IsActive.Load() {
			return p.ReverseProxy, nil
		}
	}
	return nil, errors.New("unavailable Service")
}

func (b *Balancer) DisableProxy(p *Proxy) {
	p.IsActive.Store(false)
}

func (b *Balancer) HealthCheck() {
	for _, p := range b.Proxies {
		log.Printf("Testing the server: %v\n", p.Url)
		r, err := http.Get(p.Url.String())
		if err != nil || r.StatusCode >= 500 {
			p.IsActive.Store(false)
			continue
		}

		p.IsActive.Store(true)
		if err := r.Body.Close(); err != nil {
			log.Printf("Error closing the body: %v", err)
		}
	}
}

func (b *Balancer) StartHealthCheck(ctx context.Context, d time.Duration) {
	ticker := time.NewTicker(d)
	for {
		select {
		case <-ticker.C:
			b.HealthCheck()
		case <-ctx.Done():
			ticker.Stop()
			return
		}

	}
}
