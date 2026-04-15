#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

export LD_LIBRARY_PATH="$ROOT_DIR/agora_libs${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

: "${AGORA_APP_ID:?AGORA_APP_ID is required}"
: "${AGORA_APP_CERT:?AGORA_APP_CERT is required}"
: "${DEEPSEEK_API_KEY:?DEEPSEEK_API_KEY is required}"

CHANNEL="${AGORA_CHANNEL:-demo-room}"
ASR_URL="${ASR_URL:-http://localhost:8000}"
UID_A="${UID_A:-1001}"
UID_B="${UID_B:-1002}"
LANG_A_SRC="${LANG_A_SRC:-zh}"
LANG_A_TGT="${LANG_A_TGT:-en}"
LANG_B_SRC="${LANG_B_SRC:-en}"
LANG_B_TGT="${LANG_B_TGT:-zh}"

if [[ ! -x ./go-trans ]]; then
  echo "Missing executable ./go-trans, building now..."
  CGO_LDFLAGS="-L$(pwd)/agora_libs -Wl,-rpath-link=$(pwd)/agora_libs" go build -tags rtc -o go-trans main.go
  echo "Build done: $ROOT_DIR/go-trans"
fi

if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux not found. Install it with system package manager:"
  echo "  Ubuntu/Debian: sudo apt update && sudo apt install -y tmux"
  echo "  macOS: brew install tmux"
  exit 1
fi

./scripts/start_asr.sh

SESSION="go-trans-rtc"
tmux kill-session -t "$SESSION" 2>/dev/null || true
tmux new-session -d -s "$SESSION"

CMD_A="cd '$ROOT_DIR' && export LD_LIBRARY_PATH='$ROOT_DIR/agora_libs' && AGORA_CHANNEL='$CHANNEL' AGORA_UID='$UID_A' ./go-trans rtc --role duplex --source-lang '$LANG_A_SRC' --target-lang '$LANG_A_TGT' --asr-url '$ASR_URL'"
CMD_B="cd '$ROOT_DIR' && export LD_LIBRARY_PATH='$ROOT_DIR/agora_libs' && AGORA_CHANNEL='$CHANNEL' AGORA_UID='$UID_B' ./go-trans rtc --role duplex --source-lang '$LANG_B_SRC' --target-lang '$LANG_B_TGT' --asr-url '$ASR_URL'"

tmux send-keys -t "$SESSION":0 "$CMD_A" C-m
tmux split-window -h -t "$SESSION":0
tmux send-keys -t "$SESSION":0.1 "$CMD_B" C-m

tmux select-layout -t "$SESSION":0 even-horizontal
tmux attach -t "$SESSION"
