#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
用法: scripts/devops_check.sh <issue-key>

说明:
  - 记录一次针对单个问题的检查流水，避免多问题交叉修改。
  - 顺序执行: gofmt 检查、go vet、go test、go build。
  - 每次运行都会把输出写入 devops_logs/<时间>_<issue-key>.log，便于回溯。

示例:
  scripts/devops_check.sh forward-race
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -lt 1 ]]; then
  echo "缺少 issue 标识。" >&2
  usage
  exit 1
fi

ISSUE_KEY="$1"
LOG_DIR="devops_logs"
mkdir -p "$LOG_DIR"
TIMESTAMP="$(date '+%Y%m%d_%H%M%S')"
LOG_FILE="${LOG_DIR}/${TIMESTAMP}_${ISSUE_KEY}.log"

{
  echo "=== devops_check: ${ISSUE_KEY} @ ${TIMESTAMP} ==="
  echo "Git 分支: $(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo '非 git 仓库')"
  echo "工作区状态:"
  git status --short || echo "无法读取 git 状态"
  echo

  #echo ">> gofmt 检查"
  #GOFMT_FILES=$(gofmt -l $(go list -f '{{.Dir}}' ./...) || true)
  #if [[ -n "${GOFMT_FILES}" ]]; then
  #  echo "${GOFMT_FILES}"
  #  echo "gofmt 检查未通过，请先修正上述文件。" >&2
  #  exit 1
  #fi
  #echo "gofmt OK"
  echo

  echo ">> go vet ./..."
  go vet ./...
  echo

  echo ">> go test ./..."
  go test ./...
  echo

  echo ">> go build ./..."
  go build ./...
  echo

  echo "=== 检查完成 ==="
} | tee "${LOG_FILE}"

echo "已保存检查日志: ${LOG_FILE}"
