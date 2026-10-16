package app

import (
	"os"

	"archivist/internal/documents"
	"archivist/internal/models"
	"archivist/internal/rag"
	"archivist/internal/storage"
)

type App struct {
	Store       *storage.Store
	Docs        *documents.Service
	RAG         *rag.Service
	Mode        Mode
	LLMSettings storage.LLMSettings
}

type Mode string

const (
	ModeSchool   Mode = "school"
	ModePersonal Mode = "personal"
)

func New(store *storage.Store) *App {
	settings := storage.LLMSettings{
		BaseURL:    env("OLLAMA_BASE_URL", "http://ollama:11434"),
		ChatModel:  env("ARCHIVIST_DEFAULT_MODEL", "gemma3:1b"),
		EmbedModel: env("ARCHIVIST_EMBED_MODEL", "nomic-embed-text"),
	}
	if saved, err := store.LLMSettings(settings); err == nil {
		settings = saved
	}
	client := models.New(settings.BaseURL)
	mode := Mode(env("ARCHIVIST_MODE", string(ModeSchool)))
	if mode != ModePersonal {
		mode = ModeSchool
	}
	return &App{
		Store:       store,
		Docs:        &documents.Service{DB: store.DB, UploadPath: env("ARCHIVIST_UPLOAD_PATH", "./data/uploads"), EmbedModel: settings.EmbedModel, Ollama: client},
		RAG:         &rag.Service{DB: store.DB, Ollama: client, ChatModel: settings.ChatModel, EmbedModel: settings.EmbedModel, PersonalMode: mode == ModePersonal},
		Mode:        mode,
		LLMSettings: settings,
	}
}

func (a *App) UpdateLLMSettings(settings storage.LLMSettings) error {
	if err := a.Store.SaveLLMSettings(settings); err != nil {
		return err
	}
	client := models.New(settings.BaseURL)
	a.LLMSettings = settings
	a.Docs.Ollama, a.Docs.EmbedModel = client, settings.EmbedModel
	a.RAG.Ollama, a.RAG.ChatModel, a.RAG.EmbedModel = client, settings.ChatModel, settings.EmbedModel
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
