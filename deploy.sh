#!/usr/bin/env bash
set -euo pipefail

APP_DIR="/root/GithubTelegramBot"
IMAGE="githubtelegrambot_bot:latest"
CONTAINER="githubtelegrambot_bot_1"
VOLUME="githubtelegrambot_bot-data"

cd "$APP_DIR"

echo ">>> Fetching latest code"
git fetch --prune origin
git reset --hard origin/main

echo ">>> Building image"
docker build -t "$IMAGE" .

echo ">>> Recreating container"
docker rm -f "$CONTAINER" 2>/dev/null || true
docker run -d \
  --name "$CONTAINER" \
  --restart always \
  --network host \
  --env-file "$APP_DIR/.env" \
  -v "$VOLUME":/app/data \
  "$IMAGE"

echo ">>> Pruning dangling images"
docker image prune -f >/dev/null 2>&1 || true

echo ">>> Deployed. Status:"
docker ps --filter "name=$CONTAINER" --format '{{.Names}}  {{.Status}}  {{.Image}}'
