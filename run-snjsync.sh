#!/usr/bin/env bash
# Jalankan SNJSYNC di WSL/Linux (build otomatis kalau ada perubahan kode).
cd "$(dirname "$0")"
# Matikan instance lama yang masih pegang port 8877 (kalau ada)
if pkill -x snjsync 2>/dev/null; then
  echo "== instance lama di-stop =="
  sleep 1
fi
if [ ! -f snjsync ] || [ -n "$(find . -maxdepth 1 -name '*.go' -newer snjsync -print -quit 2>/dev/null)" ]; then
  echo "== build ulang =="
  go build -o snjsync . && echo "== build selesai =="
else
  echo "== binary up-to-date =="
fi
echo "== run snjsync =="
./snjsync