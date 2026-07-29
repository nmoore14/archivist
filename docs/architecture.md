# Architecture

Archivist keeps interface code thin and shares application behavior between the web and TUI surfaces.

```text
HTTP handler / TUI command
        ↓
application service
        ↓
repository · document service · Ollama client
        ↓
view model
        ↓
HTML or terminal renderer
```

`internal/storage` owns the SQLite schema and repositories. `internal/documents` stores uploads and chunks extracted text. `internal/rag` embeds questions, ranks stored chunks with cosine similarity, builds a context-only prompt, and calls `internal/models`. Web handlers compose these services and render templates; no business rules live in the templates.

SQLite foreign keys and cascading deletes preserve workspace boundaries. Session cookies are HTTP-only and passwords use bcrypt. The Docker network is the trust boundary for Ollama; model requests never require a hosted API.
