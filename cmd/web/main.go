package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/logger"
	"github.com/waisuan/alfred/internal/router"
	"github.com/waisuan/alfred/migrations"
)

func main() {
	d, err := deps.Initialise(migrations.FS)
	if err != nil {
		logger.Fatal("init deps", logger.Err(err))
	}
	defer d.Shutdown()

	logger.Info("connected to database")

	handler := router.New(d)

	addr := "0.0.0.0:" + d.Config.Port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatal("server failed to bind", logger.Err(err))
	}

	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  d.Config.ReadTimeout,
		WriteTimeout: d.Config.WriteTimeout,
		IdleTimeout:  d.Config.IdleTimeout,
	}

	logger.Info("server listening", logger.String("addr", "http://localhost:"+d.Config.Port))
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", logger.Err(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
