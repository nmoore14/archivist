package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"archivist/internal/app"
	"archivist/internal/storage"
	"archivist/internal/web"
)

func main() {
	dbPath := env("ARCHIVIST_DB_PATH", "./data/archivist.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal(err)
	}
	store, err := storage.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.DB.Close()
	application := app.New(store)
	go maintainIndexes(context.Background(), application)
	addr := env("ARCHIVIST_HTTP_ADDR", ":8080")
	log.Printf("Archivist is ready at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, web.New(application).Handler()))
}

func maintainIndexes(ctx context.Context, application *app.App) {
	retry := time.NewTicker(5 * time.Second)
	defer retry.Stop()
	for {
		if err := application.Docs.Ollama.Health(ctx); err == nil {
			count, err := application.Docs.ReindexStale(ctx)
			if err != nil {
				log.Printf("automatic document indexing: %v", err)
			} else if count > 0 {
				log.Printf("automatically rebuilt %d outdated document indexes", count)
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-retry.C:
		}
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
