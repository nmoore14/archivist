#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/../.." && pwd)"

heading() {
  printf '\n%s\n' "Archivist guided setup"
  printf '%s\n\n' "========================"
}

choose_model() {
  printf '\nChoose the local chat model:\n'
  printf '  1) Gemma 3 1B — fastest; suitable for modest servers\n'
  printf '  2) Gemma 3 4B — better answers; requires more memory\n'
  read -r -p "Model [1]: " model_choice
  case "${model_choice:-1}" in
    1) CHAT_MODEL="gemma3:1b" ;;
    2) CHAT_MODEL="gemma3:4b" ;;
    *)
      echo "That is not a valid model choice." >&2
      exit 1
      ;;
  esac
}

ask_lan() {
  printf '\nEnter the network allowed to open Archivist.\n'
  printf 'For many small networks this is 192.168.1.0/24.\n'
  read -r -p "Allowed network [192.168.1.0/24]: " LAN_CIDR
  LAN_CIDR="${LAN_CIDR:-192.168.1.0/24}"
  if [[ ! "$LAN_CIDR" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}/[0-9]{1,2}$ ]]; then
    echo "Use IPv4 CIDR notation, such as 192.168.1.0/24." >&2
    exit 1
  fi
}

run_linux_install() {
  local installer="$1"
  if [[ "$(uname -s)" != "Linux" ]]; then
    echo "The production installer must be run on the Linux server that will host Archivist." >&2
    exit 1
  fi
  ask_lan
  choose_model
  printf '\nArchivist will be available only to %s after installation.\n' "$LAN_CIDR"
  printf 'The installer needs internet access temporarily to build images and download models.\n'
  read -r -p "Continue? [y/N]: " confirm
  case "$confirm" in
    y|Y|yes|YES)
      sudo env \
        LAN_CIDR="$LAN_CIDR" \
        ARCHIVIST_DEFAULT_MODEL="$CHAT_MODEL" \
        ARCHIVIST_EMBED_MODEL="nomic-embed-text" \
        "$installer"
      ;;
    *) echo "Setup cancelled." ;;
  esac
}

heading
printf 'What would you like to do?\n'
printf '  1) Full Linux install — install prerequisites and Archivist\n'
printf '  2) Archivist-only install — Docker and Nginx are already installed\n'
printf '  3) Portable demo — nested container for demonstrations\n'
printf '  4) Build only — prepare standard container images\n'
printf '  5) Verify — confirm an installed Linux deployment is offline\n'
printf '  q) Quit\n'
read -r -p "Choice: " choice

cd "$PROJECT_DIR"
case "$choice" in
  1) run_linux_install "$SCRIPT_DIR/bootstrap-ubuntu.sh" ;;
  2) run_linux_install "$SCRIPT_DIR/install.sh" ;;
  3)
    choose_model
    echo "Building and starting the portable demo..."
    ARCHIVIST_DEFAULT_MODEL="$CHAT_MODEL" \
      docker compose -f deploy/demo-parent/compose.yml up --build -d --wait
    ./deploy/demo-parent/verify.sh
    echo "Open http://localhost:8080"
    ;;
  4)
    echo "Building Archivist container images..."
    docker compose build
    ;;
  5)
    sudo ./deploy/linux/verify.sh
    ;;
  q|Q) echo "No changes made." ;;
  *)
    echo "That is not a valid choice." >&2
    exit 1
    ;;
esac
