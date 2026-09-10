# Bundled Ollama models

This directory is an Ollama model store, not an application model definition.
It contains the manifests and immutable blobs for Archivist's default models:

- `gemma3:1b`
- `nomic-embed-text`

`models/Dockerfile` copies this store into the custom Ollama image. On startup,
`entrypoint.sh` merges it into `/root/.ollama/models`, which lets the existing
`ollama-data` volume persist model files without hiding the bundled cache.

Do not rename or edit files in `blobs/` or `manifests/`; Ollama resolves model
names from that exact layout. To package a different model, pull it with
Ollama once on a development machine and copy its manifest plus every digest it
references into the corresponding folders here.
