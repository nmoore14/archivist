#!/usr/bin/env bash
set -euo pipefail

INNER_PROJECT="archivist-demo-inner"
INNER_COMPOSE="/workspace/deploy/linux/compose.yml"
BOOTSTRAP_CONTAINER="archivist-demo-model-bootstrap"
CHAT_MODEL="${ARCHIVIST_DEFAULT_MODEL:-gemma3:1b}"
EMBED_MODEL="${ARCHIVIST_EMBED_MODEL:-nomic-embed-text}"
DOCKER_LOG="/var/log/archivist-demo-dockerd.log"
DOCKERD_PID=""
NGINX_PID=""

shutdown() {
  local exit_code=$?
  trap - EXIT TERM INT

  if [[ -n "$NGINX_PID" ]]; then
    kill -TERM "$NGINX_PID" >/dev/null 2>&1 || true
    wait "$NGINX_PID" 2>/dev/null || true
  fi

  if [[ -n "$DOCKERD_PID" ]]; then
    kill -TERM "$DOCKERD_PID" >/dev/null 2>&1 || true
    wait "$DOCKERD_PID" 2>/dev/null || true
  fi

  exit "$exit_code"
}

cleanup_bootstrap() {
  docker rm -f "$BOOTSTRAP_CONTAINER" >/dev/null 2>&1 || true
}

echo "Starting the nested Docker daemon..."
dockerd-entrypoint.sh dockerd --host=unix:///var/run/docker.sock >"$DOCKER_LOG" 2>&1 &
DOCKERD_PID=$!
trap shutdown EXIT TERM INT

for _ in {1..60}; do
  if docker info >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker info >/dev/null 2>&1 || {
  echo "Nested Docker failed to start:" >&2
  tail -n 100 "$DOCKER_LOG" >&2
  exit 1
}

echo "Building the nested Archivist application..."
docker compose -p "$INNER_PROJECT" -f "$INNER_COMPOSE" build

echo "Preparing the local Ollama models..."
docker volume create "${INNER_PROJECT}_ollama-data" >/dev/null
cleanup_bootstrap
docker run -d \
  --name "$BOOTSTRAP_CONTAINER" \
  -v "${INNER_PROJECT}_ollama-data:/root/.ollama" \
  ollama/ollama:latest serve >/dev/null

for _ in {1..60}; do
  if docker exec "$BOOTSTRAP_CONTAINER" ollama list >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker exec "$BOOTSTRAP_CONTAINER" ollama show "$CHAT_MODEL" >/dev/null 2>&1; then
  docker exec "$BOOTSTRAP_CONTAINER" ollama pull "$CHAT_MODEL"
fi
if ! docker exec "$BOOTSTRAP_CONTAINER" ollama show "$EMBED_MODEL" >/dev/null 2>&1; then
  docker exec "$BOOTSTRAP_CONTAINER" ollama pull "$EMBED_MODEL"
fi
cleanup_bootstrap

echo "Starting Archivist and Ollama on the nested internal network..."
docker compose -p "$INNER_PROJECT" -f "$INNER_COMPOSE" up -d --wait

echo "Blocking new outbound connections from the demo parent..."
iptables -I OUTPUT 1 -o lo -j ACCEPT
iptables -I OUTPUT 2 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
iptables -I OUTPUT 3 -d 172.30.0.0/24 -j ACCEPT
iptables -P OUTPUT DROP

if ip6tables -L OUTPUT >/dev/null 2>&1; then
  ip6tables -I OUTPUT 1 -o lo -j ACCEPT
  ip6tables -I OUTPUT 2 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
  ip6tables -P OUTPUT DROP
fi
touch /run/archivist-demo-egress-locked

echo "Demo ready on port 8080. New parent and child outbound traffic is blocked."
nginx -g "daemon off;" &
NGINX_PID=$!
wait "$NGINX_PID"
