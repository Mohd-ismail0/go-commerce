package main

import (
	"log"

	"rewrite/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
