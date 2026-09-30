#!/usr/bin/env bash
# Cross-compiles IkeGawa for Windows using Docker + MinGW-w64 — no local
# Go/gcc needed.
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Building Windows image"
docker build -t ikegawa-windows-builder -f build/Dockerfile.windows .

echo "==> Compiling Windows binary"
docker run --rm \
  -v "$(pwd)":/src \
  -v ikegawa-gomodcache:/go/pkg/mod \
  -w /src \
  -e GOOS=windows -e GOARCH=amd64 -e CGO_ENABLED=1 \
  -e CC=x86_64-w64-mingw32-gcc \
  -e CGO_LDFLAGS="-static-libgcc -static-libstdc++" \
  ikegawa-windows-builder \
  sh -c "go build -buildvcs=false -ldflags='-H=windowsgui' -o build/ikegawa-windows-amd64.exe ."

echo "==> Fixing file ownership (container runs as root)"
docker run --rm -v "$(pwd)":/src alpine chown -R "$(id -u):$(id -g)" /src/build /src/go.mod /src/go.sum

echo "==> Done:"
ls -la build/ikegawa-windows-amd64.exe
