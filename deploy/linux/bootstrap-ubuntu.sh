#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

if [[ $EUID -ne 0 ]]; then
  echo "Run this bootstrap with sudo." >&2
  exit 1
fi

if [[ -z "${LAN_CIDR:-}" ]]; then
  echo "Set LAN_CIDR to the network allowed to use Archivist." >&2
  echo "Example: sudo LAN_CIDR=192.168.1.0/24 $0" >&2
  exit 1
fi

. /etc/os-release
case "${ID:-}" in
  ubuntu|debian) ;;
  *)
    echo "This bootstrap supports Ubuntu and Debian. Use install.sh on other Linux distributions." >&2
    exit 1
    ;;
esac

apt-get update
apt-get install -y ca-certificates curl nginx

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL "https://download.docker.com/linux/$ID/gpg" \
    -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc

  dpkg_arch="$(dpkg --print-architecture)"
  . /etc/os-release
  echo \
    "deb [arch=$dpkg_arch signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/$ID ${VERSION_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list

  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
fi

exec "$SCRIPT_DIR/install.sh"
