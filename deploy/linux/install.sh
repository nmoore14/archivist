#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/compose.yml"
PROJECT_NAME="archivist"
LAN_CIDR="${LAN_CIDR:-}"
CHAT_MODEL="${ARCHIVIST_DEFAULT_MODEL:-gemma3:1b}"
EMBED_MODEL="${ARCHIVIST_EMBED_MODEL:-nomic-embed-text}"
BACKEND_SUBNET="${ARCHIVIST_BACKEND_SUBNET:-172.30.0.0/24}"
APP_IP="${ARCHIVIST_APP_IP:-172.30.0.10}"
OLLAMA_IP="${ARCHIVIST_OLLAMA_IP:-172.30.0.11}"
BOOTSTRAP_CONTAINER="archivist-model-bootstrap"
DOCKER_BIN=""

if [[ $EUID -ne 0 ]]; then
  echo "Run this installer with sudo." >&2
  exit 1
fi

if [[ -z "$LAN_CIDR" ]]; then
  echo "Set LAN_CIDR to the network allowed to use Archivist." >&2
  echo "Example: sudo LAN_CIDR=192.168.1.0/24 $0" >&2
  exit 1
fi
if [[ ! "$LAN_CIDR" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}/[0-9]{1,2}$ ]]; then
  echo "LAN_CIDR must be an IPv4 CIDR such as 192.168.1.0/24." >&2
  exit 1
fi

for command in docker nginx curl sed systemctl; do
  if ! command -v "$command" >/dev/null; then
    echo "Missing required command: $command" >&2
    exit 1
  fi
done

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is required." >&2
  exit 1
fi
DOCKER_BIN="$(command -v docker)"

cleanup_bootstrap() {
  docker rm -f "$BOOTSTRAP_CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup_bootstrap EXIT

echo "Building Archivist while installation-time internet access is available..."
docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" build --pull

echo "Preloading Ollama models into the persistent model volume..."
if systemctl is-active --quiet archivist.service; then
  systemctl stop archivist.service
else
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down
fi
docker volume create "${PROJECT_NAME}_ollama-data" >/dev/null
cleanup_bootstrap
docker run -d \
  --name "$BOOTSTRAP_CONTAINER" \
  -v "${PROJECT_NAME}_ollama-data:/root/.ollama" \
  ollama/ollama:latest serve >/dev/null

for _ in {1..30}; do
  if docker exec "$BOOTSTRAP_CONTAINER" ollama list >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$BOOTSTRAP_CONTAINER" ollama pull "$CHAT_MODEL"
docker exec "$BOOTSTRAP_CONTAINER" ollama pull "$EMBED_MODEL"
cleanup_bootstrap

echo "Installing the LAN reverse proxy..."
sed -e "s|__LAN_CIDR__|$LAN_CIDR|g" \
  -e "s|__ARCHIVIST_APP_IP__|$APP_IP|g" \
  "$SCRIPT_DIR/nginx.conf.template" \
  > /etc/nginx/conf.d/archivist.conf
nginx -t
systemctl enable --now nginx
systemctl reload nginx

echo "Installing the Archivist system service..."
install -d -m 0755 /etc/archivist
{
  printf 'ARCHIVIST_DEFAULT_MODEL=%s\n' "$CHAT_MODEL"
  printf 'ARCHIVIST_EMBED_MODEL=%s\n' "$EMBED_MODEL"
  printf 'ARCHIVIST_BACKEND_SUBNET=%s\n' "$BACKEND_SUBNET"
  printf 'ARCHIVIST_APP_IP=%s\n' "$APP_IP"
  printf 'ARCHIVIST_OLLAMA_IP=%s\n' "$OLLAMA_IP"
} > /etc/archivist/archivist.env
chmod 0644 /etc/archivist/archivist.env
sed -e "s|__PROJECT_DIR__|$PROJECT_DIR|g" \
  -e "s|__DOCKER_BIN__|$DOCKER_BIN|g" \
  "$SCRIPT_DIR/archivist.service.template" \
  > /etc/systemd/system/archivist.service
systemctl daemon-reload
systemctl enable --now archivist.service

echo "Waiting for Archivist to become healthy..."
for _ in {1..60}; do
  if curl --fail --silent --max-time 2 http://127.0.0.1:8080/ >/dev/null; then
    break
  fi
  sleep 2
done

"$SCRIPT_DIR/verify.sh"

SERVER_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Archivist is ready for the LAN at http://${SERVER_IP:-SERVER_IP}:8080"
echo "Runtime containers are isolated from the internet."
