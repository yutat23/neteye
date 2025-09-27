#!/bin/bash
# Cross-platform build script for neteye

set -euo pipefail

# Configuration
BINARY_NAME="neteye"
VERSION=${VERSION:-"dev"}
COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
DATE=${DATE:-$(date -u '+%Y-%m-%d_%H:%M:%S')}

# Build flags
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/386"
    "windows/amd64"
    "windows/386"
    "darwin/amd64"
    "darwin/arm64"
)

# Create dist directory
mkdir -p dist

echo "Building neteye ${VERSION} (${COMMIT}) for multiple platforms..."

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$platform"
    
    output_name="${BINARY_NAME}-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "Building for ${GOOS}/${GOARCH}..."
    
    env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
        go build -ldflags="${LDFLAGS}" \
        -o "dist/${output_name}" \
        ./cmd/neteye
    
    if [ $? -ne 0 ]; then
        echo "Failed to build for ${GOOS}/${GOARCH}"
        exit 1
    fi
    
    echo "✓ Built: dist/${output_name}"
done

echo ""
echo "Build complete! Binaries are in dist/"
ls -la dist/

# Create checksums
echo ""
echo "Generating checksums..."
cd dist
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum * > checksums.txt
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 * > checksums.txt
else
    echo "Warning: No checksum utility found"
fi

if [ -f checksums.txt ]; then
    echo "✓ Checksums saved to dist/checksums.txt"
    cat checksums.txt
fi

cd ..

echo ""
echo "All builds completed successfully!"
