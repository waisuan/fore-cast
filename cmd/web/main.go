package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/waisuan/alfred/internal/config"
	"github.com/waisuan/alfred/internal/router"
	"github.com/waisuan/alfred/internal/saujana"
	"github.com/waisuan/alfred/internal/session"
)

func main() {
	cfg := config.Load()
	store := session.NewStore(cfg.SessionTTL)
	saujanaClient := saujana.NewClient()
	handler := router.New(cfg, store, saujanaClient)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
