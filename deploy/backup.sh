#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE=${ENV_FILE:-"$SCRIPT_DIR/.env.production"}
COMPOSE_FILE="$SCRIPT_DIR/compose.production.yaml"
BACKUP_DIR=${BACKUP_DIR:-"$SCRIPT_DIR/backups"}

if [ ! -f "$ENV_FILE" ]; then
    echo "missing production environment: $ENV_FILE" >&2
    exit 1
fi

set -a
. "$ENV_FILE"
set +a
umask 077
mkdir -p "$BACKUP_DIR"
OUTPUT="$BACKUP_DIR/postgres-$(date -u +%Y%m%dT%H%M%SZ).dump"

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T db \
    pg_dump --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --format=custom --no-owner --no-acl > "$OUTPUT"

test -s "$OUTPUT"
echo "$OUTPUT"
