#!/bin/sh
set -e

echo "Entrypoint: starting up..."

/root/setup.sh
service named start || true
/root/dyndns &
DYNDNS_PID=$!

shutdown() {
  echo "Entrypoint: received signal, forwarding to children..."

  service named stop || true
  kill -TERM "$DYNDNS_PID" 2>/dev/null || true

  wait
}

# Trap SIGTERM/SIGINT
trap 'shutdown' TERM INT

wait "$DYNDNS_PID"