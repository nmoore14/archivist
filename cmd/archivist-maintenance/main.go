package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"archivist/internal/app"
	"archivist/internal/storage"
)

func main() {
	if len(os.Args) < 3 || (os.Args[1] != "reindex" && os.Args[1] != "ask") {
		fmt.Fprintln(os.Stderr, "usage: archivist-maintenance reindex <workspace-id>")
		fmt.Fprintln(os.Stderr, "       archivist-maintenance ask <workspace-id> <question>")
		os.Exit(2)
	}
	workspaceID, err := strconv.ParseInt(os.Args[2], 10, 64)
	if err != nil || workspaceID < 1 {
		log.Fatal("workspace ID must be a positive integer")
	}
	dbPath := env("ARCHIVIST_DB_PATH", "./data/archivist.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatal(err)
	}
	store, err := storage.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.DB.Close()
	application := app.New(store)
	if os.Args[1] == "ask" {
		if len(os.Args) < 4 {
			log.Fatal("question is required")
		}
		answer, sources, err := application.RAG.Answer(context.Background(), workspaceID, strings.Join(os.Args[3:], " "))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(answer)
		fmt.Printf("\nRetrieved sources: %s\n", strings.Join(sources, "; "))
		return
	}
	if err := application.Docs.ReindexWorkspace(context.Background(), workspaceID); err != nil {
		log.Fatal(err)
	}
	log.Printf("workspace %d reindexed successfully", workspaceID)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
