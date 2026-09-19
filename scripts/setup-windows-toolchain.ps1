#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$ToolchainRoot = Join-Path $RepoRoot ".toolchain\ucrt64-gcc14"
$MsysBash = "C:\msys64\usr\bin\env.exe"
if (-not (Test-Path $MsysBash)) {
    throw "MSYS2 not found at C:\msys64. Install from https://www.msys2.org/ and run: pacman -S mingw-w64-ucrt-x86_64-toolchain"
}

$setupSh = @'
set -euo pipefail
root="__REPO_ROOT__/.toolchain/ucrt64-gcc14"
base="https://mirror.msys2.org/mingw/ucrt64"
ver="14.2.0-2"
mkdir -p "$root"
cd "$root"
for pkg in \
  "mingw-w64-ucrt-x86_64-gcc-libs-${ver}-any.pkg.tar.zst" \
  "mingw-w64-ucrt-x86_64-gcc-${ver}-any.pkg.tar.zst"; do
  if [[ ! -f "$pkg" ]]; then
    curl -fL -o "$pkg" "$base/$pkg"
  fi
  tar -xf "$pkg"
done
pacman -Sw --noconfirm \
  mingw-w64-ucrt-x86_64-crt \
  mingw-w64-ucrt-x86_64-binutils \
  mingw-w64-ucrt-x86_64-headers \
  mingw-w64-ucrt-x86_64-winpthreads
for pkg in /var/cache/pacman/pkg/mingw-w64-ucrt-x86_64-{crt,binutils,headers,winpthreads}-*.pkg.tar.zst; do
  tar -xf "$pkg" -C "$root"
done
if [[ ! -f /ucrt64/lib/default-manifest.o ]]; then
  echo "default-manifest.o missing from MSYS2 UCRT64; update mingw-w64-ucrt-x86_64-crt" >&2
  exit 1
fi
cp /ucrt64/lib/default-manifest.o "$root/ucrt64/lib/default-manifest.o"
test -x "$root/ucrt64/bin/gcc.exe"
echo "Toolchain ready at $root/ucrt64"
'@

$setupSh = $setupSh.Replace("__REPO_ROOT__", ($RepoRoot -replace '\\', '/'))
$tempSh = Join-Path $env:TEMP "agentsview-setup-toolchain.sh"
Set-Content -Path $tempSh -Value $setupSh -Encoding UTF8
& $MsysBash MSYSTEM=UCRT64 C:\msys64\usr\bin\bash.exe -lc "bash '$($tempSh -replace '\\', '/')'"

Write-Host "Windows CGO toolchain installed under $ToolchainRoot"
