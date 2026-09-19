#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
toolchain="$repo_root/.toolchain/ucrt64-gcc14/ucrt64"
if [[ ! -x "$toolchain/bin/gcc.exe" ]]; then
  echo "Missing $toolchain/bin/gcc.exe" >&2
  echo "Run scripts/setup-windows-toolchain.ps1 first." >&2
  exit 1
fi
export PATH="$toolchain/bin:/c/Program Files/Go/bin:$PATH"
export CC="$toolchain/bin/gcc.exe"
export CXX="$toolchain/bin/g++.exe"
export CGO_ENABLED=1
unset CGO_CFLAGS CGO_LDFLAGS CGO_CXXFLAGS
cd "$repo_root"
go build -tags fts5 -o agentsview.exe ./cmd/agentsview/
