package app

import (
	"os"

	"archivist/internal/documents"
	"archivist/internal/models"
	"archivist/internal/rag"
	"archivist/internal/storage"
)

type App struct {
	Store *storage.Store
	Docs  *documents.Service
	RAG   *rag.Service
}

func New(store *storage.Store) *App {
	base := env("OLLAMA_BASE_URL", "http://ollama:11434")
	client := models.New(base)
	return &App{
		Store: store,
		Docs:  &documents.Service{DB: store.DB, UploadPath: env("ARCHIVIST_UPLOAD_PATH", "./data/uploads"), EmbedModel: env("ARCHIVIST_EMBED_MODEL", "nomic-embed-text"), Ollama: client},
		RAG:   &rag.Service{DB: store.DB, Ollama: client, ChatModel: env("ARCHIVIST_DEFAULT_MODEL", "gemma3:1b"), EmbedModel: env("ARCHIVIST_EMBED_MODEL", "nomic-embed-text")},
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
