#!/bin/bash
# 清理proxy_configs下的日志文件，只保留1天
# 每天凌晨执行

LOG_DIR="/root/port-forward/proxy_configs"

# 清理logs_*目录下超过1天的日志文件
find "$LOG_DIR"/logs_* -type f -mtime +1 -delete 2>/dev/null

# 清理空目录（可选）
# find "$LOG_DIR"/logs_* -type d -empty -delete 2>/dev/null

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Proxy logs cleanup completed"
