package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"geofence/internal/config"
	"geofence/internal/server"
	"geofence/internal/store"
	"geofence/internal/ws"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-health" {
		os.Exit(probeHealth())
	}

	cfg := config.Load()
	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	if cfg.Seed {
		created, err := st.Seed(context.Background())
		if err != nil {
			log.Fatalf("seed: %v", err)
		}
		if created {
			log.Printf("seeded sample zones and vehicles")
		}
	}

	hub := ws.New()
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.New(st, hub, cfg).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("harbor API listening on %s (database %s)", httpServer.Addr, cfg.DatabasePath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func probeHealth() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
