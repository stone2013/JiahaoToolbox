#!/usr/bin/env sh
# Development cross-build. Runtime use of the EXE requires no developer tools.
set -eu
cd "$(dirname "$0")/.."
version=$(go env GOVERSION)
minor=$(printf '%s' "$version" | cut -d. -f2)
if [ "$minor" -lt 26 ] && [ "${ALLOW_LEGACY_TOOLCHAIN:-0}" != 1 ]; then
  echo 'Use a supported, fully patched Go release. ALLOW_LEGACY_TOOLCHAIN=1 is for evaluation reproduction only.' >&2; exit 1
fi
# Caller must verify current security support; this floor is not a vulnerability scan.
go run ./cmd/resourcegen
[ -f frontend/dist/assets/main.js ] || node scripts/build-frontend.mjs
mkdir -p build
stamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -H=windowsgui -X jiahao-toolbox/internal/app.Version=4.2.0 -X jiahao-toolbox/internal/app.BuildTime=$stamp" -o build/JiahaoToolbox_v4.2.exe ./cmd/jiahao-toolbox
(cd build && sha256sum JiahaoToolbox_v4.2.exe > SHA256SUMS.txt)
printf '%s\n' 'Cross-build complete. Native Windows validation is still required.'
