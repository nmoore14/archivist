#!/bin/sh
set -eu

# A named Docker volume masks image files at /root/.ollama. Seed any missing
# bundled files before Ollama starts, while retaining models added later.
mkdir -p /root/.ollama/models
cp -a -n /opt/archivist-models/. /root/.ollama/models/

exec /bin/ollama "$@"
