#!/bin/bash
# release.sh - Build release binaries for multiple platforms
# Usage: ./scripts/release.sh [version]

set -euo pipefail

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')

LDFLAGS="-X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}"

echo "Building sqlquest ${VERSION}"
echo "Build date: ${BUILD_DATE}"
echo "Git commit: ${GIT_COMMIT}"

# Clean previous builds
rm -rf dist/
mkdir -p dist/

# Build for multiple platforms
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    OUTPUT="dist/sqlquest_${VERSION}_${GOOS}_${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT="${OUTPUT}.exe"
    fi
    
    echo "Building for ${GOOS}/${GOARCH}..."
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "${LDFLAGS}" -o "${OUTPUT}" ./cmd/sqlquest
done

# Create checksums
echo "Generating checksums..."
cd dist/
sha256sum * > checksums.txt
cd ..

echo ""
echo "Release artifacts in dist/:"
ls -la dist/

echo ""
echo "Done! Version: ${VERSION}"