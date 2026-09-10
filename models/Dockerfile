FROM ollama/ollama:latest

# Keep the bundled model cache outside Ollama's writable directory. The
# entrypoint merges it into the persistent volume when the container starts.
COPY models/blobs/ /opt/archivist-models/blobs/
COPY models/manifests/ /opt/archivist-models/manifests/
COPY models/entrypoint.sh /usr/local/bin/archivist-ollama-entrypoint

RUN chmod 755 /usr/local/bin/archivist-ollama-entrypoint

ENTRYPOINT ["/usr/local/bin/archivist-ollama-entrypoint"]
CMD ["serve"]
