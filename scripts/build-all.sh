#!/usr/bin/env bash
set -e

mkdir -p build

export CGO_ENABLED=0
echo "Building binaries..."

go tool dist list | grep -vE 'wasm|aix|plan9|android|ios|illumos|solaris|dragonfly' | while IFS=/ read -r GOOS GOARCH; do
    echo "Building for $GOOS/$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build -v -o build/workspaced-$GOOS-$GOARCH ./cmd/workspaced || echo "Failed to build for $GOOS/$GOARCH"
done

echo "Build complete. Artifacts in build/"
ls -lh build/
