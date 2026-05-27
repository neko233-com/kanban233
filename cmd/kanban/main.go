package main

import (
	"flag"
	"log"

	"github.com/neko233/kanban233/internal/config"
	"github.com/neko233/kanban233/internal/server"
)

func main() {
	configPath := flag.String("config", server.ConfigPath(), "path to server.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	defer srv.Close()

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
