# Running Archivist

This guide covers every supported way to run Archivist, from a private study helper on one computer to a LAN-accessible Linux installation.

## Choose a run mode

| Goal | Recommended command or guide |
| --- | --- |
| Personal, login-free study helper with bundled Ollama | [Personal study mode](#personal-study-mode-with-bundled-ollama) |
| School/workspace mode with accounts and bundled Ollama | [Standard Docker mode](#standard-docker-mode) |
| Use an Ollama server on your host PC or another machine | [External Ollama](#external-ollama-server) |
| Develop from source with Go and a local Ollama installation | [Local Go development](#local-go-development) |
| LAN-accessible, offline-capable Linux server | [Linux deployment](#linux-lan-deployment) |
| Portable demonstration of the Linux deployment | [Nested demo](#nested-container-demo) |

## Prerequisites

For Docker-based modes, install Docker Engine or Docker Desktop with Docker Compose v2. Docker is the only host dependency for the bundled stack.

For local development, install Go 1.23 or newer and Ollama.

Run all commands from the repository root:

```sh
cd /Users/nickmoore/Desktop/school/summer_2026/msai_699/archivist
```

## Standard Docker mode

This runs Archivist and the repository’s bundled Ollama service. It starts in school/workspace mode, where the first visitor creates an administrator account.

```sh
docker compose up --build -d
```

Open <http://localhost:8080>.

On first use:

1. Create the administrator account.
2. Create a workspace.
3. Upload `.txt`, `.md`, `.html`, `.htm`, or text-based `.pdf` material.
4. Wait for material to finish indexing, then ask grounded questions.

## Personal study mode with bundled Ollama

Personal mode removes login, accounts, and people management. It creates one local owner automatically and treats workspaces as private study spaces.

```sh
ARCHIVIST_MODE=personal \
docker compose up --build -d
```

Open <http://localhost:8080>.

Keep the default `ARCHIVIST_HOST_BIND=127.0.0.1` in this mode. A login-free server must not be exposed directly to your LAN or the internet.

To begin studying:

1. Create a study space.
2. Upload material and give the batch a type and optional tags.
3. Use **Study with Archivist** for explanations, summaries, comparisons, guides, study plans, practice questions, and flashcards.
4. Visit **Saved study** to revisit generated material and **Review due cards** for flashcards.

## External Ollama server

Use this if Ollama runs on the Docker host, on a more powerful PC, or on another reachable computer. This Compose file starts only Archivist; it does not start the bundled Ollama container.

First, stop a standard stack if it is already using port 8080:

```sh
docker compose down
```

Then start Archivist using the external endpoint:

```sh
ARCHIVIST_MODE=personal \
OLLAMA_BASE_URL=http://host.docker.internal:11434 \
ARCHIVIST_DEFAULT_MODEL=gemma3:1b \
ARCHIVIST_EMBED_MODEL=nomic-embed-text \
docker compose -f docker-compose.external-ollama.yml up --build -d
```

Open <http://localhost:8080>.

`host.docker.internal` reaches Ollama on the Docker host with Docker Desktop and modern Linux Docker. For Ollama on another computer, replace it with that machine’s reachable hostname or IP address, for example:

```sh
OLLAMA_BASE_URL=http://192.168.1.25:11434 \
docker compose -f docker-compose.external-ollama.yml up --build -d
```

The external Ollama service must be reachable from the Archivist container and have both the configured chat and embedding models installed. It may need to listen on a network interface, for example with `OLLAMA_HOST=0.0.0.0:11434`.

After starting the app, **Settings** in the left rail can change the endpoint and both model names. Saved settings override environment defaults and persist across restarts.

## Local Go development

Start Ollama locally and ensure the configured models are available. Then run Archivist directly:

```sh
ARCHIVIST_MODE=personal \
OLLAMA_BASE_URL=http://127.0.0.1:11434 \
go run ./cmd/archivist-server
```

Open <http://localhost:8080>. Add `ARCHIVIST_DEFAULT_MODEL` and `ARCHIVIST_EMBED_MODEL` if your local Ollama uses different models.

The supplied `.env.example` lists all supported environment variables. Copy it to `.env` when you want a repeatable local configuration.

## Linux LAN deployment

Use this for a school or business Linux server. Archivist and Ollama run on an internal Docker network; host Nginx is the only LAN-facing component.

For a guided setup menu:

```sh
make setup
```

For a clean Ubuntu/Debian server, install Docker, Nginx, and Archivist in one command:

```sh
sudo LAN_CIDR=192.168.1.0/24 ./deploy/linux/bootstrap-ubuntu.sh
```

If the prerequisites are already installed:

```sh
sudo LAN_CIDR=192.168.1.0/24 ./deploy/linux/install.sh
```

Replace `192.168.1.0/24` with your permitted LAN subnet. Open `http://SERVER_LAN_IP:8080` from a device on that network. See [Linux deployment](linux-deployment.md) for verification, upgrades, operations, and removal.

## Nested container demo

This is a portable demonstration of the Linux deployment. It requires a privileged Docker-in-Docker parent container and is not for production.

```sh
docker compose -f deploy/demo-parent/compose.yml up --build -d
docker compose -f deploy/demo-parent/compose.yml logs -f
```

Wait for `Demo ready on port 8080`, then open `http://HOST_LAN_IP:8080`.

Verify the demo’s isolation:

```sh
docker exec archivist-demo-parent verify.sh
```

See [Nested container demo](nested-demo.md) for security notes and cleanup instructions.

## Everyday Docker operations

For the standard bundled stack:

```sh
docker compose ps
docker compose logs -f
docker compose down
```

The last command stops containers but keeps the `archivist-data` and `ollama-data` volumes. Your workspaces, documents, and models return on the next start.

Common Make targets are also available:

```sh
make build
make status
make logs
make stop
make test
```

Use the matching Compose file for commands against an external Ollama stack, for example:

```sh
docker compose -f docker-compose.external-ollama.yml logs -f
docker compose -f docker-compose.external-ollama.yml down
```

## Optional maintenance tools

Open the terminal interface inside a running bundled Docker stack:

```sh
docker compose exec archivist archivist
```

Rebuild the index for workspace `1`:

```sh
docker compose exec archivist archivist-maintenance reindex 1
```

Replace `1` with the workspace ID you want to rebuild.

## Troubleshooting

- **The page does not load:** confirm port 8080 is free and run `docker compose ps`.
- **Archivist cannot reach a model:** follow logs, confirm the Ollama endpoint is reachable, and verify both configured models are installed.
- **External Ollama fails from Docker:** use `host.docker.internal` for a host-machine server, or use the reachable LAN IP/DNS name for another machine. Make sure the Ollama server listens beyond loopback when it is on another machine.
- **Uploads do not answer questions yet:** wait until the library marks the document as ready; indexing must complete before it becomes retrieval context.
- **Scanned PDFs have no useful text:** Archivist currently supports text-based PDFs; scanned/image-only PDF OCR is not implemented.
