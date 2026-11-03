package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reverse-proxy/proxy"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()

	urls := []string{"http://localhost:8001", "http://localhost:8002", "http://localhost:8003"}
	balancer := proxy.NewBalancer(urls)
	mux.HandleFunc("GET /", balancer.Handler)

	server := &http.Server{
		Addr:              ":8000",
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Println("Server initialized successfully.")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Panicf("Error starting the server %v", err)
			return
		}
	}()

	go func() {
		balancer.StartHealthCheck(ctx, 10*time.Second)
	}()

	<-ctx.Done()

	gracefulCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(gracefulCtx); err != nil {
		log.Panicf("Error closing the server %v", err)
	}

	log.Println("Server closed successfully.")

}
