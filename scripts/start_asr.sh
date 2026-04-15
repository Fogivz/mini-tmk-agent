#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR/asr_service"

PORT="${ASR_PORT:-8000}"

if command -v ss >/dev/null 2>&1 && ss -ltn | awk '{print $4}' | grep -Eq ":${PORT}$"; then
  echo "ASR already listening on port ${PORT}"
  exit 0
fi

if [[ ! -d venv ]]; then
  python3 -m venv venv
fi

source venv/bin/activate

if [[ -f requirements.txt ]]; then
  pip install -r requirements.txt >/dev/null
fi

nohup python -m uvicorn app:app --host 0.0.0.0 --port "$PORT" >"$ROOT_DIR/asr_service/asr.log" 2>&1 &

echo "ASR started on port ${PORT}, log: $ROOT_DIR/asr_service/asr.log"
