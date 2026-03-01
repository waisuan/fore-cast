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

	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/router"
	"github.com/waisuan/alfred/migrations"
)

func main() {
	d, err := deps.Initialise(migrations.FS)
	if err != nil {
		log.Fatalf("init deps: %v", err)
	}
	defer d.Shutdown()

	log.Println("Connected to database")

	handler := router.New(d)

	addr := "0.0.0.0:" + d.Config.Port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Server failed to bind: %v", err)
	}

	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  d.Config.ReadTimeout,
		WriteTimeout: d.Config.WriteTimeout,
		IdleTimeout:  d.Config.IdleTimeout,
	}

	log.Printf("Server listening on http://localhost:%s", d.Config.Port)
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
