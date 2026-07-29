#!/usr/bin/env bash
set -euo pipefail

INNER_PROJECT="archivist-demo-inner"
INNER_COMPOSE="/workspace/deploy/linux/compose.yml"
INNER_NETWORK="${INNER_PROJECT}_backend"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "PASS: $*"
}

if [[ ! -f /run/archivist-demo-egress-locked ]]; then
  if pgrep -f "entrypoint.sh" >/dev/null 2>&1; then
    echo "WAIT: first-time initialization is still running." >&2
    echo "Follow progress with: docker logs -f archivist-demo-parent" >&2
    exit 2
  fi
  fail "initialization stopped before the parent outbound firewall was applied"
fi
pass "parent outbound firewall was applied"

[[ "$(docker network inspect "$INNER_NETWORK" --format '{{.Internal}}')" == "true" ]] \
  || fail "nested backend network is not internal"
pass "nested backend network is internal"

for service in archivist ollama; do
  container_id="$(docker compose -p "$INNER_PROJECT" -f "$INNER_COMPOSE" ps -q "$service")"
  [[ -n "$container_id" ]] || fail "$service child container is missing"
  [[ "$(docker inspect "$container_id" --format '{{.State.Running}}')" == "true" ]] \
    || fail "$service child container is not running"

  if docker exec "$container_id" sh -c \
    "awk 'NR > 1 && \$2 == \"00000000\" { found=1 } END { exit found ? 0 : 1 }' /proc/net/route"; then
    fail "$service child has a default outbound route"
  fi
  pass "$service child has no default outbound route"
done

curl --fail --silent --show-error --max-time 5 http://127.0.0.1:8080/ >/dev/null \
  || fail "parent Nginx cannot reach nested Archivist"
pass "parent Nginx reaches nested Archivist"

if timeout 3 bash -c 'exec 3<>/dev/tcp/1.1.1.1/80' 2>/dev/null; then
  fail "parent can create an outbound internet connection"
fi
pass "parent outbound connection attempt was blocked"

echo "Nested demo verification passed."
