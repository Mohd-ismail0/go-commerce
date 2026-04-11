package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"rewrite/internal/app"
)

// Injected at link time, e.g. go build -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(appCtx)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	log.Printf("rewrite commerce engine v%s listening on :%s", strings.TrimSpace(version), listenPort())

	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server failed: %v", err)
		}
	case <-appCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := application.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("shutdown failed: %v", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("server failed: %v", err)
		}
	}
}

func listenPort() string {
	if p := strings.TrimSpace(os.Getenv("PORT")); p != "" {
		return p
	}
	return "8080"
}
