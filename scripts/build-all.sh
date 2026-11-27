#!/usr/bin/env bash
# Build semua target: desktop, linux/arm64 (Termux), Android APK.
set -e
cd "$(dirname "$0")/.."

echo "▸ desktop"
go build -o dist/grokbot ./cmd/grokbot

echo "▸ linux/arm64 (Termux/HP)"
GOOS=linux GOARCH=arm64 go build -o dist/grokbot-arm64 ./cmd/grokbot

echo "▸ windows"
GOOS=windows GOARCH=amd64 go build -o dist/grokbot.exe ./cmd/grokbot

if command -v gomobile >/dev/null; then
  echo "▸ android"
  ./scripts/build-android.sh
else
  echo "▸ android dilewati (gomobile belum terinstall)"
fi
echo "✓ selesai → dist/"
