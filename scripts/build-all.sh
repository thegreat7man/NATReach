#!/usr/bin/env bash
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.25+ is required: https://go.dev/dl/" >&2
  exit 1
fi

version="${VERSION:-0.1.0}"
root="$(cd "$(dirname "$0")/.." && pwd)"
dist="$root/dist"
mkdir -p "$dist"
rm -f "$dist"/natreach-"$version"-*.tar.gz "$dist"/natreach-"$version"-*.zip "$dist/SHA256SUMS"

build() {
  os="$1"
  arch="$2"
  ext="$3"
  name="natreach-${version}-${os}-${arch}"
  stage="$(mktemp -d "${TMPDIR:-/tmp}/natreach-build.XXXXXX")"
  binary="$stage/natreach$ext"
  echo "Building $name"
  (cd "$root" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$binary" .)
  cp "$root/README.md" "$root/LICENSE" "$stage/"
  if [ "$os" = "darwin" ]; then
    cp "$root/assets/NATReach.command" "$stage/"
    chmod +x "$stage/NATReach.command"
  fi
  if [ "$os" = "windows" ]; then
    (cd "$stage" && zip -q -r "$dist/$name.zip" .)
  else
    tar -C "$stage" -czf "$dist/$name.tar.gz" .
  fi
  rm -rf "$stage"
}

build darwin amd64 ""
build darwin arm64 ""
build linux amd64 ""
build linux arm64 ""
build windows amd64 ".exe"
build windows arm64 ".exe"

(
  cd "$dist"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum natreach-"$version"-*.tar.gz natreach-"$version"-*.zip
  else
    shasum -a 256 natreach-"$version"-*.tar.gz natreach-"$version"-*.zip
  fi
) > "$dist/SHA256SUMS"

echo "Done: $dist"
