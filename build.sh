#!/usr/bin/env bash
# Builds IkeGawa for Linux and Windows using Docker — no local Go/gcc needed.
# macOS cannot be reliably cross-compiled from Linux (Apple SDK licensing);
# see README.md for how to build it on an actual Mac.
set -euo pipefail
cd "$(dirname "$0")"

./build-linux.sh
./build-windows.sh
