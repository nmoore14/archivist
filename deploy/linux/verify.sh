#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/compose.yml"
PROJECT_NAME="archivist"
NETWORK_NAME="${PROJECT_NAME}_backend"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "PASS: $*"
}

[[ "$(docker network inspect "$NETWORK_NAME" --format '{{.Internal}}')" == "true" ]] \
  || fail "$NETWORK_NAME is not internal"
pass "Docker backend network is internal"

for service in archivist ollama; do
  container_id="$(docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" ps -q "$service")"
  [[ -n "$container_id" ]] || fail "$service container is missing"
  [[ "$(docker inspect "$container_id" --format '{{.State.Running}}')" == "true" ]] \
    || fail "$service container is not running"
  port_bindings="$(docker inspect "$container_id" --format '{{json .HostConfig.PortBindings}}')"
  [[ "$port_bindings" == "{}" || "$port_bindings" == "null" ]] \
    || fail "$service publishes a host port"

  if docker exec "$container_id" sh -c \
    "awk 'NR > 1 && \$2 == \"00000000\" { found=1 } END { exit found ? 0 : 1 }' /proc/net/route"; then
    fail "$service has a default outbound route"
  fi
  pass "$service has no default outbound route"
done
pass "runtime containers publish no host ports"

curl --fail --silent --show-error --max-time 5 http://127.0.0.1:8080/ >/dev/null \
  || fail "Nginx cannot reach Archivist"
pass "host reverse proxy reaches Archivist"

echo "Offline Linux deployment verification passed."
