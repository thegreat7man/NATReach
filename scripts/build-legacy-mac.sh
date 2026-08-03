#!/usr/bin/env bash
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.24.13 is required for the Big Sur builds." >&2
  exit 1
fi

export GOTOOLCHAIN=local

version="${VERSION:-0.1.0}"
root="$(cd "$(dirname "$0")/.." && pwd)"
dist="$root/dist"
mkdir -p "$dist"

go_version="$(go env GOVERSION)"
case "$go_version" in
  go1.24.*) ;;
  *) echo "Legacy builds must use Go 1.24.x; found $go_version" >&2; exit 1 ;;
esac

for arch in amd64 arm64; do
  name="natreach-${version}-darwin-bigsur-${arch}"
  stage="$(mktemp -d "${TMPDIR:-/tmp}/natreach-legacy.XXXXXX")"
  echo "Building $name with $go_version"
  (cd "$root" && CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -modfile=go.legacy.mod -trimpath -ldflags "-s -w -X main.version=$version-legacy" -o "$stage/natreach" .)
  cp "$root/README.md" "$root/LICENSE" "$root/assets/NATReach.command" "$stage/"
  chmod +x "$stage/natreach" "$stage/NATReach.command"
  tar -C "$stage" -czf "$dist/$name.tar.gz" .
  rm -rf "$stage"
done

(
  cd "$dist"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum natreach-"$version"-*.tar.gz natreach-"$version"-*.zip
  else
    shasum -a 256 natreach-"$version"-*.tar.gz natreach-"$version"-*.zip
  fi
) > "$dist/SHA256SUMS"

echo "Legacy macOS builds added to $dist"
