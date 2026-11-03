package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func startBackendServer(port int, name string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"message": "Alive the %s", "port": %d}`, name, port)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("[%s] Running on http://localhost:%d\n", name, port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[%s] Stopped: %v\n", name, err)
		}
	}()

	return server
}

func main() {
	log.Println("Starting mock backend servers...")

	servers := []*http.Server{
		startBackendServer(8001, "server-1"),
		startBackendServer(8002, "server-2"),
		startBackendServer(8003, "server-3"),
	}

	// Wait for shutdown signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("\nShutting down all mock backend servers...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, s := range servers {
		if err := s.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down server %s: %v\n", s.Addr, err)
		}
	}

	log.Println("All mock servers closed successfully.")
}
