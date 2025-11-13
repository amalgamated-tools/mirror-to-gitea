#!/usr/bin/env sh

set -e

# Get custom delay, else use 3600 seconds
DELAY="${DELAY:-3600}"

while true
do
  echo "Starting to create mirrors..."
  /app/mirror-to-gitea

  case $SINGLE_RUN in
    (TRUE | true | 1) break;;
  esac

  echo "Waiting for ${DELAY} seconds..."
  sleep "${DELAY}"
done
