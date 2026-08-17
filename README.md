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

## Run Archivist with Docker

Before you begin, install Docker Engine with the Docker Compose v2 plugin. Docker
is the only host dependency.

To start Archivist locally:

```sh
docker compose up --build -d
```

Download the chat and embedding models:

```sh
docker compose exec ollama ollama pull gemma3:1b
docker compose exec ollama ollama pull nomic-embed-text
```

Open the [Archivist web interface](http://localhost:8080). On a new data volume,
Archivist displays **Create Admin Account**. Create the first administrator
account, and then create a workspace, upload course sources, create student
accounts, and assign the students to the workspace.

Archivist extracts, chunks, and embeds new uploads immediately. It records the
indexing-pipeline version for each document. On startup, Archivist waits for
Ollama and rebuilds missing, failed, or outdated document indexes.

To follow the application logs:

```sh
docker compose logs -f
```

For detailed setup instructions and troubleshooting, see [Set up Archivist
locally](docs/setup.md). Run `make help` to list the common build, test, status,
log, and shutdown commands.

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

You can change the defaults with the variables in the [environment variable
example](.env.example).

## Offline Linux deployment

The production-style Linux deployment keeps both runtime containers on an
internal Docker network with no published ports. Nginx on the host is the only
LAN-facing component. It proxies port `8080` to Archivist.

For installation, verification, upgrade, and removal instructions, see [Deploy
Archivist on an offline Linux network](docs/linux-deployment.md).

For a portable, single-parent-container demonstration, see [Run the nested
container demo](docs/nested-demo.md). That privileged Docker-in-Docker wrapper
is separate from the production deployment.

## Current MVP limitations

- Text-based PDFs are extracted page by page with page-level citations. Scanned or image-only PDFs require a future OCR workflow.
- Embeddings are stored as JSON in SQLite and scanned in-process; this favors inspectability over scale.
- Reindex is a route and UI foundation; background job processing is not yet implemented.
- Student password reset, account editing, CSRF tokens, rate limiting, and production TLS are future hardening work.
- Chat history is currently grouped by workspace rather than separate named conversations.

## Evaluate model behavior

Archivist includes a reproducible no-RAG versus RAG evaluation with 15 versioned
questions, automated screening, latency measurements, and a manual-review
template. See [Run the model evaluation](evaluation/README.md) for prerequisites,
commands, scoring guidance, and generated artifacts.

## Roadmap

1. Add OCR and layout-aware extraction for scanned and complex PDFs.
2. Move ingestion and reindexing into observable background jobs.
3. Add conversation sessions and richer source previews.
4. Add admin model-health and setup guidance.
5. Evaluate guided setup task completion, time, and error rates for the capstone study.
