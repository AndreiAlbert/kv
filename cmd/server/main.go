package main

import (
	"kv-store/internal/config"
	"kv-store/internal/server"
	"log"
)

func main() {
	cfg := config.NewFromFlags()
	srv := server.NewServer(cfg)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
