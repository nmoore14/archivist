# Archivist

Archivist is a local-first AI workspace for schools. An administrator creates course workspaces and uploads trusted source material; assigned students ask questions and receive answers grounded in that material, with source references.

The capstone investigates whether guided workflows can reduce the technical barriers to configuring and deploying local AI assistants for educational use.

## Stack

- Go HTTP server and shared application services
- SQLite for users, sessions, course content, messages, jobs, and audit records
- Server-rendered `html/template` views with HTMX-style partial updates
- Ollama for local embeddings and chat inference
- Bubble Tea TUI foundation
- Docker Compose for a two-service local deployment

## Run with Docker

Docker is the only required host dependency.

```sh
docker compose up --build
```

In another terminal, install the local models:

```sh
docker compose exec ollama ollama pull gemma3:1b
docker compose exec ollama ollama pull nomic-embed-text
```

Open [http://localhost:8080](http://localhost:8080). On a fresh data volume, Archivist opens the **Create Admin Account** screen. Create the first admin, then add a workspace, course sources, student accounts, and workspace assignments.

New uploads are extracted, chunked, and embedded immediately. Archivist also records the indexing-pipeline version used for each document. On startup it waits for Ollama, detects missing, failed, or outdated document indexes, and rebuilds only those documents automatically.

To open the TUI inside the running app container:

```sh
docker compose exec archivist archivist
```

To rebuild all document embeddings for a workspace from the command line:

```sh
docker compose exec archivist archivist-maintenance reindex 1
```

## Local development

With Go 1.23 and Ollama installed:

```sh
go run ./cmd/archivist-server
```

Defaults may be changed with the variables documented in [.env.example](.env.example).

## Offline Linux deployment

The production-style Linux deployment keeps both runtime containers on an
internal Docker network with no published ports. Host Nginx is the only
LAN-facing component and proxies port 8080 to Archivist.

See [docs/linux-deployment.md](docs/linux-deployment.md) for the installation,
demo, verification, upgrade, and removal workflow.

For a portable, single-parent-container demonstration, see
[docs/nested-demo.md](docs/nested-demo.md). That privileged Docker-in-Docker
wrapper is intentionally separate from the production deployment.

## Current MVP limitations

- Text-based PDFs are extracted page by page with page-level citations. Scanned or image-only PDFs require a future OCR workflow.
- Embeddings are stored as JSON in SQLite and scanned in-process; this favors inspectability over scale.
- Reindex is a route and UI foundation; background job processing is not yet implemented.
- Student password reset, account editing, CSRF tokens, rate limiting, and production TLS are future hardening work.
- Chat history is currently grouped by workspace rather than separate named conversations.

## Roadmap

1. Add OCR and layout-aware extraction for scanned and complex PDFs.
2. Move ingestion and reindexing into observable background jobs.
3. Add conversation sessions and richer source previews.
4. Add admin model-health and setup guidance.
5. Evaluate guided setup task completion, time, and error rates for the capstone study.
