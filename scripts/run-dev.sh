#!/usr/bin/env bash
# Jalankan night desk (dev): go run + auto-recompile tiap perubahan kode.
set -e
cd "$(dirname "$0")/.."
exec go run ./cmd/grokbot "$@"
