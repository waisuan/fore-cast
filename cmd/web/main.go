package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/config"
	"github.com/waisuan/alfred/internal/router"
	"github.com/waisuan/alfred/internal/session"
)

func main() {
	cfg := config.Load()
	store := session.NewStore(cfg.SessionTTL)
	client := booker.NewClient()
	handler := router.New(cfg, store, client)

	addr := "0.0.0.0:" + cfg.Port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Server failed to bind: %v", err)
	}

	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	log.Printf("Server listening on http://localhost:%s", cfg.Port)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
