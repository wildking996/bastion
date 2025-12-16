#!/bin/bash

echo "Building Bastion V3 for multiple platforms..."
echo ""

mkdir -p dist

# Check if rsrc tool is available for embedding icon
if command -v rsrc &> /dev/null; then
    echo "Generating Windows resources with icon..."
    rsrc -ico icon.ico -o rsrc.syso
    if [ $? -ne 0 ]; then
        echo "Warning: Failed to generate resources, continuing without icon..."
    fi
else
    echo "Note: rsrc tool not found. Install with: go install github.com/akavel/rsrc@latest"
    echo "Building without icon..."
fi

echo "[1/5] Building for Windows (amd64) - GUI mode (no console)..."
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H windowsgui" -o dist/bastion-windows-amd64.exe
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "[2/5] Building for Windows (amd64) - Console mode..."
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/bastion-windows-amd64-console.exe
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "[3/5] Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o dist/bastion-linux-amd64
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "[4/5] Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o dist/bastion-darwin-amd64
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "[5/5] Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o dist/bastion-linux-arm64
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo ""
echo "========================================"
echo "Build completed successfully!"
echo "========================================"
echo ""
echo "Output files:"
ls -lh dist/bastion-*
echo ""