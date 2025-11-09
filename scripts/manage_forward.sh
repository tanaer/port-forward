#!/usr/bin/env bash
set -euo pipefail

BIN_DEFAULT="./goForward"
BIN="${GOFORWARD_BIN:-$BIN_DEFAULT}"
PORT="${GOFORWARD_PORT:-8889}"
PASS="${GOFORWARD_PASS:-qq123456}"
PID_FILE="${GOFORWARD_PID:-/tmp/goforward.pid}"
LOG_DIR="${GOFORWARD_LOGDIR:-./logs}"
LOG_FILE="${LOG_DIR}/goForward.log"

usage() {
  cat <<EOF
用法: scripts/manage_forward.sh {start|stop|restart|status}

环境变量:
  GOFORWARD_BIN     可执行文件路径，默认 ${BIN_DEFAULT}
  GOFORWARD_PORT    Web 端口，默认 8889
  GOFORWARD_PASS    登录密码，默认 qq123456
  GOFORWARD_PID     PID 文件路径，默认 /tmp/goforward.pid
  GOFORWARD_LOGDIR  日志目录，默认 ./logs
EOF
}

ensure_bin() {
  if [[ ! -x "$BIN" ]]; then
    echo "未找到可执行文件: $BIN" >&2
    exit 1
  fi
}

ensure_logdir() {
  mkdir -p "$LOG_DIR"
}

is_running() {
  [[ -f "$PID_FILE" ]] || return 1
  local pid
  pid=$(<"$PID_FILE")
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

start() {
  if is_running; then
    echo "进程已在运行 (PID $(<"$PID_FILE"))."
    return 0
  fi

  ensure_bin
  ensure_logdir

  nohup "$BIN" -port "$PORT" -pass "$PASS" >>"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
  echo "已启动 goForward，PID $(<"$PID_FILE")，日志输出到 $LOG_FILE"
}

stop() {
  if ! is_running; then
    echo "进程未运行。"
    [[ -f "$PID_FILE" ]] && rm -f "$PID_FILE"
    return 0
  fi
  local pid
  pid=$(<"$PID_FILE")
  kill "$pid" 2>/dev/null || true
  for i in {1..10}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      rm -f "$PID_FILE"
      echo "已停止进程 $pid"
      return 0
    fi
    sleep 1
  done
  echo "进程 $pid 未在 10 秒内退出，尝试强制停止"
  kill -9 "$pid" 2>/dev/null || true
  rm -f "$PID_FILE"
}

status() {
  if is_running; then
    echo "运行中，PID $(<"$PID_FILE"), 日志: $LOG_FILE"
  else
    echo "未运行。"
  fi
}

restart() {
  stop
  sleep 1
  start
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

case "$1" in
  start) start ;;
  stop) stop ;;
  restart) restart ;;
  status) status ;;
  *) usage; exit 1 ;;
esac
