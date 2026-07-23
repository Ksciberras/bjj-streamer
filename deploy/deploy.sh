#!/bin/sh
set -eu

REPOSITORY_DIR=${REPOSITORY_DIR:-/srv/bjj-streaming}
ENV_FILE="$REPOSITORY_DIR/deploy/.env.production"
COMPOSE_FILE="$REPOSITORY_DIR/deploy/compose.production.yaml"

if [ -z "${DEPLOY_SHA:-}" ]; then
    echo "DEPLOY_SHA is required" >&2
    exit 1
fi
if [ ! -f "$ENV_FILE" ]; then
    echo "missing production environment: $ENV_FILE" >&2
    exit 1
fi

cd "$REPOSITORY_DIR"
git fetch origin main
git cat-file -e "$DEPLOY_SHA^{commit}"

./deploy/backup.sh

git switch main
git merge --ff-only "$DEPLOY_SHA"

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" config --quiet
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build --wait
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps
