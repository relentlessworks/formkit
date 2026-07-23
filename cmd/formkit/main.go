package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/relentlessworks/formkit/internal/api"
	"github.com/relentlessworks/formkit/internal/auth"
	"github.com/relentlessworks/formkit/internal/config"
	"github.com/relentlessworks/formkit/internal/store"
)

func main() {
	cfg := config.Load()
	cfg.Sanitize()

	// Initialize store
	s, err := store.New(cfg.Data)
	if err != nil {
		log.Fatalf("Failed to initialize data store: %v", err)
	}

	// Initialize auth
	a := auth.New(s, cfg.SMTP)

	// Initialize API handler
	h := api.NewHandler(s, a)

	// Set up server
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      h.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		_ = srv.Close()
	}()

	fmt.Fprintf(os.Stderr, "FormKit listening on %s (data: %s)\n", cfg.Addr, cfg.Data)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
