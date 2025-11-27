package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourorg/head/internal/config"
	"github.com/yourorg/head/internal/metrics"
	"github.com/yourorg/head/internal/server"
)

func main() {
	cfg := config.Load()

	go metrics.Start(cfg.MetricsPort)

	srv := server.New(cfg)

	serveErr := make(chan error,1)
	go func() {
		serveErr <- srv.Run()
	}()

	sig := make(chan os.Signal,1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case s := <-sig:
		log.Printf("signal %v, shutting down", s)
	case err := <-serveErr:
		log.Printf("server error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
