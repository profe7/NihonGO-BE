package main

import (
	"context"
	"embed"
	"log"
	"time"

	"nihongo/internal/api"
	"nihongo/internal/config"
	"nihongo/internal/db"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()
	log.Println("connected to database")

	if err := db.RunMigrations(ctx, pool, migrationsFS); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}
	log.Println("migrations up to date")

	r := api.NewRouter(pool, cfg)

	addr := "0.0.0.0:" + cfg.Port
	log.Printf("NihonGO API listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
