#!/bin/sh
set -eu

if [ "$#" -ne 2 ] || [ "$1" != "--confirm" ]; then
    echo "usage: $0 --confirm BACKUP.dump" >&2
    echo "This replaces the application database contents." >&2
    exit 2
fi

BACKUP_FILE=$2
if [ ! -s "$BACKUP_FILE" ]; then
    echo "backup is missing or empty: $BACKUP_FILE" >&2
    exit 1
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE=${ENV_FILE:-"$SCRIPT_DIR/.env.production"}
COMPOSE_FILE="$SCRIPT_DIR/compose.production.yaml"

if [ ! -f "$ENV_FILE" ]; then
    echo "missing production environment: $ENV_FILE" >&2
    exit 1
fi

set -a
. "$ENV_FILE"
set +a

restart_application() {
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d api web caddy
}
trap restart_application EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" stop api web
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T db \
    pg_restore --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --clean --if-exists --no-owner --no-acl --exit-on-error < "$BACKUP_FILE"

echo "restore completed from $BACKUP_FILE"
