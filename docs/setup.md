# Setup

1. Start the app with `docker compose up --build`.
2. Pull `gemma3:1b` and `nomic-embed-text` into the Ollama service.
3. Open `http://localhost:8080`.
4. Create the initial administrator account.
5. Create and optionally publish a course workspace.
6. Add `.txt`, `.md`, or `.pdf` course sources.
7. Create student accounts and assign them from a workspace overview.

Data persists in the `archivist-data` volume. Models persist in `ollama-data`. Environment defaults are listed in `.env.example`.
