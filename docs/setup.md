# Setup

Use this procedure to run Archivist on your computer for development or
evaluation. For a LAN-accessible installation that runs without internet access
after setup, see [Deploy Archivist on an offline Linux
network](linux-deployment.md).

## Before you begin

Install Docker Engine with the Docker Compose v2 plugin. The first setup needs
internet access only to obtain container images and application dependencies:
the required Ollama models are bundled in the repository's `models/` directory.

Port `8080` must be available on the host.

## Start the services

1. From the repository root, build and start Archivist and Ollama:

   ```sh
   docker compose up --build -d
   ```

2. Open the [Archivist web interface](http://localhost:8080).

3. On the **Create Admin Account** screen, create the initial administrator
   account.

4. Create a workspace. You can publish it immediately or leave it unpublished
   while you add content.

5. Upload one or more `.txt`, `.md`, `.html`, `.htm`, or `.pdf` files.

6. Create student accounts and assign the students from the workspace overview.

Text-based PDF files support page-level citations. Archivist doesn't extract
text from scanned or image-only PDF files.

## Check the services

To list the running containers:

```sh
docker compose ps
```

To follow logs while Archivist indexes a document or answers a question:

```sh
docker compose logs -f
```

If a model command reports that Ollama is unavailable, wait for the `ollama`
container to finish starting, and then run the command again.

## Stop the services

```sh
docker compose down
```

This command keeps application data in the `archivist-data` volume and models
in the `ollama-data` volume. The next start reuses both volumes.

## Configure the environment

To override a default, copy the repository's [environment variable
example](../.env.example) to `.env` and edit the value before you start the
services. Keep `OLLAMA_BASE_URL` set to `http://ollama:11434` when Ollama runs
through the provided Docker Compose configuration.
