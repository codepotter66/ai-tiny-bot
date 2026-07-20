#!/bin/sh
# ============================================================
# tiny-bot-cloud-agent container entrypoint
# ============================================================
# 跑两件事：
#   1. 修挂载目录权限（named volume / bind mount 常是 root:root，USER tinybot 写不了）
#   2. 用 su-exec 把进程降权到 tinybot 再 exec 二进制
#
# 不直接 USER tinybot 的原因：
#   - USER 之后所有命令都以 tinybot 跑，chown 没权限
#   - 用 su-exec 在 entrypoint 内降权，标准 Docker 模式
# ============================================================

set -e

# Named volume：SQLite 等
mkdir -p /opt/tiny-bot/data
chown -R tinybot:tinybot /opt/tiny-bot/data 2>/dev/null || true

# Bind mount workspace：情节日志 / facts.md 需要可写
mkdir -p /opt/tiny-bot/workspace
chown -R tinybot:tinybot /opt/tiny-bot/workspace 2>/dev/null || true

# 把所有参数（通常是 CMD 里的 binary 路径）以 tinybot 身份执行
exec su-exec tinybot "$@"
