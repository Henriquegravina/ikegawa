#!/usr/bin/env bash
# Builds IkeGawa for Linux using Docker — no local Go/gcc needed.
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Building Linux image"
docker build -t ikegawa-linux-builder -f build/Dockerfile.linux .

echo "==> Compiling Linux binary"
docker run --rm \
  -v "$(pwd)":/src \
  -v ikegawa-gomodcache:/go/pkg/mod \
  -w /src \
  ikegawa-linux-builder \
  sh -c "go build -buildvcs=false -o build/ikegawa-linux-amd64 ."

echo "==> Fixing file ownership (container runs as root)"
docker run --rm -v "$(pwd)":/src alpine chown -R "$(id -u):$(id -g)" /src/build /src/go.mod /src/go.sum

echo "==> Done:"
ls -la build/ikegawa-linux-amd64
