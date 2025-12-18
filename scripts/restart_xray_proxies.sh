#!/usr/bin/env bash

set -euo pipefail

LOG_FILE="/tmp/restart_xray.log"
PATTERN="xray run -config proxy_configs/xray_"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

mkdir -p "$(dirname "$LOG_FILE")"
exec > >(tee -a "$LOG_FILE") 2>&1

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting Xray proxy restart."

cd "$REPO_ROOT"

mapfile -t configs < <(
  ps aux \
    | grep "$PATTERN" \
    | grep -v grep \
    | grep -oE "proxy_configs/xray_[^ ]+" \
    | sort -u
)

if [[ ${#configs[@]} -eq 0 ]]; then
  echo "No running Xray proxies found for pattern '$PATTERN'."
  exit 0
fi

echo "Found ${#configs[@]} Xray proxy configuration(s)."

for cfg in "${configs[@]}"; do
  echo " - $cfg"
done

if ! pkill -f "$PATTERN"; then
  echo "pkill did not terminate any processes."
fi

echo "Waiting for processes to exit..."
sleep 2

for cfg in "${configs[@]}"; do
  echo "Restarting proxy with $cfg"
  nohup ./bin/xray run -config "$cfg" >> "$LOG_FILE" 2>&1 &
done

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Restart complete."
