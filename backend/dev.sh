#!/bin/bash
# Runs just the backend locally (no containers, no mysql/prometheus) against
# whatever DB_DSN/FILES_PASSWORD/MACHINES_PASSWORD are provided in backend/.env
# or already exported in the shell. See deploy.sh for the full pod-based flow.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

ENV_FILE="$SCRIPT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

# main.go/session.go read these straight from the environment (db.go,
# session.go) — fail fast here instead of letting the backend start and
# reject every DB call / password check one request at a time.
missing=()
for var in DB_DSN FILES_PASSWORD MACHINES_PASSWORD; do
  if [ -z "${!var:-}" ]; then
    missing+=("$var")
  fi
done

if [ "${#missing[@]}" -gt 0 ]; then
  echo "Missing required env var(s): ${missing[*]}" >&2
  echo "Set them in backend/.env (gitignored) or export them before running this script." >&2
  exit 1
fi

exec go run .
