#!/usr/bin/env bash
# Build Libredesk (Vue frontend + Go backend) into the single libredesk/libredesk binary.
# Run this after changing code, then restart dev/start.sh.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/libredesk"
# The Makefile reads the version from git tags, which this repo doesn't carry
export LIBREDESK_VERSION="v$(sed -n 's/.*tag: v\([0-9.]*\).*/\1/p' VERSION | head -1)"
[ "$LIBREDESK_VERSION" = "v" ] && export LIBREDESK_VERSION="v2.9.2-kd"
make build
