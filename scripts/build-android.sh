#!/usr/bin/env bash
# Build APK Android: gomobile bind (AAR) + Gradle assembleDebug.
set -e
cd "$(dirname "$0")/.."

if ! command -v gomobile >/dev/null; then
  echo "install dulu: go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init"
  exit 1
fi

echo "▸ build AAR (arm64)…"
mkdir -p android/app/libs
gomobile bind -target=android/arm64 -androidapi 24 \
  -o android/app/libs/botlib.aar ./android/botlib

echo "▸ build APK…"
cd android
if [ -x ./gradlew ]; then
  ./gradlew assembleDebug
else
  gradle assembleDebug
fi
echo "✓ APK: android/app/build/outputs/apk/debug/app-debug.apk"
