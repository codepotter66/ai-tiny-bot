#!/bin/bash
# ============================================================
# tiny-bot-cloud-agent - Mac 端一键部署脚本
# ============================================================
# 跟 update-tiny-bot-cloud-agent.sh（远端）配套使用
#
# 用法：
#   ./deploy/deploy.sh                          # 用 .env 里默认的 SERVER
#   ./deploy/deploy.sh user@host                 # 指定 server
#   ./deploy/deploy.sh user@host /path/to/dir    # 指定 server + dir
#
# 流程：
#   1) docker save 把镜像打成 tar
#   2) scp tar + 部署脚本到服务器
#   3) ssh 触发 update-tiny-bot-cloud-agent.sh --from-tar
#   4) 远端健康检查 + 返回结果
#
# 前置：服务器上 daemon.json 镜像源已配（首次跑过 daemon 部署后即可）
# ============================================================

set -e

# 锁定项目根目录：让 .env / .env.example / docker-compose.yml 路径稳定
# （不管从哪个目录跑 make deploy）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# 从 .env 安全读取部署相关变量（只匹配键名，不 source 整文件）
load_env_key() {
	local key="$1"
	local file="$2"
	[ -f "$file" ] || return 0
	local line
	line=$(grep -E "^${key}=" "$file" | head -1 || true)
	[ -n "$line" ] || return 0
	printf '%s\n' "${line#*=}"
}

if [ -z "${DEPLOY_SERVER:-}" ]; then
	DEPLOY_SERVER="$(load_env_key DEPLOY_SERVER .env)"
fi
if [ -z "${DEPLOY_SERVER_DIR:-}" ]; then
	DEPLOY_SERVER_DIR="$(load_env_key DEPLOY_SERVER_DIR .env)"
fi

DEFAULT_TAR="tiny-bot-cloud-agent.tar"
DEFAULT_IMAGE="tiny-bot-cloud-agent:dev"
CADDY_TAR="tiny-bot-caddy.tar"
CADDY_IMAGE="tiny-bot-caddy:2-alpine"

SERVER="${1:-${DEPLOY_SERVER:-}}"
SERVER_DIR="${2:-${DEPLOY_SERVER_DIR:-}}"
TAR_FILE="${3:-$DEFAULT_TAR}"
IMAGE="${4:-$DEFAULT_IMAGE}"

if [ -z "$SERVER" ]; then
	echo "❌ 未设置 DEPLOY_SERVER"
	echo "   请在 .env 中设置：DEPLOY_SERVER=user@host"
	echo "   或：./deploy/deploy.sh user@host /path/to/deploy/root"
	echo "   或：make deploy SERVER=user@host SERVER_DIR=/path/to/deploy/root"
	exit 1
fi

if [ -z "$SERVER_DIR" ]; then
	echo "❌ 未设置 DEPLOY_SERVER_DIR"
	echo "   请在 .env 中设置：DEPLOY_SERVER_DIR=/path/to/deploy/root"
	echo "   或：./deploy/deploy.sh user@host /path/to/deploy/root"
	echo "   或：make deploy SERVER=user@host SERVER_DIR=/path/to/deploy/root"
	exit 1
fi

# 解析 server 主机名（用于健康检查）
SERVER_HOST=$(echo "$SERVER" | cut -d@ -f2)

# ============================================================
# 0) v1.1.7: 顶部 .env 状态横幅（让用户一眼看到）
# ============================================================
echo "============================================="
echo "  部署配置"
echo "============================================="
echo "  Server:    $SERVER"
echo "  ServerDir: $SERVER_DIR"
if [ -f .env ]; then
    ENV_SIZE=$(wc -c < .env)
    echo "  .env:      ✅ 存在（$ENV_SIZE bytes，将随部署一起推）"
    # 关键 provider 状态速览
    if grep -qE "^TB_PROVIDER_LLM=openai_compat" .env; then
        echo "  LLM:       ✅ openai_compat（会读真 LLM）"
    elif grep -qE "^TB_PROVIDER_LLM=mock" .env; then
        echo "  LLM:       ⚠️  mock（不会读真 LLM；改 openai_compat）"
    fi
    if grep -qE "^TB_PROVIDER_TTS=minimax" .env; then
        echo "  TTS:       ✅ minimax（会读真 TTS）"
    fi
else
    echo "  .env:      ⚠️  不存在（远端会从 .env.example 复制首次版本）"
    echo "             → 跑 'cp .env.example .env && vi .env' 创建本地 .env"
fi
echo "============================================="
echo ""

# ============================================================
# 0) 校验镜像在本地
# ============================================================
# 用 docker images -q 查镜像 ID（比 inspect 宽松，能处理多 registry prefix）
if ! docker images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -qx "$IMAGE"; then
    echo "❌ 本地没有镜像 $IMAGE"
    echo ""
    echo "本地有哪些镜像："
    docker images 2>&1 | grep -i "tiny-bot\|REPOSITORY" || docker images | head -10
    echo ""
    echo "请先 build："
    echo "  docker build --platform linux/amd64 -f docker/Dockerfile -t $IMAGE ."
    exit 1
fi

# ============================================================
# 1) docker save 导出 tar
# ============================================================
echo "============================================="
echo "▶ 1/5 导出镜像 tar（agent + 自建 caddy，分两个 tar）..."
echo "============================================="
if ! docker images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -qx "$CADDY_IMAGE"; then
    echo "  构建 Caddy 镜像..."
    docker build --platform linux/amd64 --provenance=false --sbom=false \
        -f docker/Dockerfile.caddy -t "$CADDY_IMAGE" .
fi
echo "  保存 agent → $TAR_FILE"
docker save "$IMAGE" -o "$TAR_FILE"
echo "  保存 caddy → $CADDY_TAR"
docker save "$CADDY_IMAGE" -o "$CADDY_TAR"
TAR_SIZE=$(du -h "$TAR_FILE" | cut -f1)
CADDY_SIZE=$(du -h "$CADDY_TAR" | cut -f1)
echo "  ✅ $TAR_FILE ($TAR_SIZE)"
echo "  ✅ $CADDY_TAR ($CADDY_SIZE)"

# v1.1.9: 打包项目文件（docker-compose.yml + workspace/ + entrypoint.sh）
# 服务器需要这些文件才能跑 docker compose up，光有镜像 tar 不够
FILES_TAR="tiny-bot-cloud-agent-files.tar.gz"
echo ""
echo "  打包项目文件 → $FILES_TAR ..."
tar -czf "$FILES_TAR" \
    --exclude='data/*' \
    --exclude='bin/*' \
    --exclude='.env' \
    docker-compose.yml .env.example workspace demo deploy/entrypoint.sh deploy/Caddyfile deploy/Caddyfile.selfsigned deploy/caddy-selfsigned-entrypoint.sh
FILES_SIZE=$(du -h "$FILES_TAR" | cut -f1)
echo "  ✅ $FILES_TAR ($FILES_SIZE)"

# ============================================================
# 2) scp 两个 tar 到服务器
# ============================================================
echo ""
echo "============================================="
echo "▶ 2/5 scp tar + files 到 $SERVER:$SERVER_DIR/"
echo "============================================="
scp "$TAR_FILE" "$SERVER:$SERVER_DIR/"
scp "$CADDY_TAR" "$SERVER:$SERVER_DIR/"
scp "$FILES_TAR" "$SERVER:$SERVER_DIR/"

# ============================================================
# 3) scp 部署脚本
# ============================================================
echo ""
echo "============================================="
echo "▶ 3/5 scp 部署脚本到 $SERVER"
echo "============================================="
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
scp "$SCRIPT_DIR/update-tiny-bot-cloud-agent.sh" "$SERVER:$SERVER_DIR/update-tiny-bot-cloud-agent.sh"

# ============================================================
# 4) ssh 触发远端部署（先 extract + load + up，目录就有了）
# ============================================================
echo ""
echo "============================================="
echo "▶ 4/5 触发远端部署（extract files + load image + compose up）..."
echo "============================================="
ssh "$SERVER" "chmod +x $SERVER_DIR/update-tiny-bot-cloud-agent.sh && cd $SERVER_DIR && ./update-tiny-bot-cloud-agent.sh --from-tar $SERVER_DIR/$TAR_FILE --caddy-tar $SERVER_DIR/$CADDY_TAR --files $SERVER_DIR/$FILES_TAR"

# ============================================================
# 5) scp .env（现在项目目录已经存在！）
# ============================================================
echo ""
echo "============================================="
echo "▶ 5/5 scp .env 到 $SERVER (项目目录已就绪)"
echo "============================================="
if [ -f .env ]; then
    scp .env "$SERVER:$SERVER_DIR/tiny-bot-cloud-agent/.env"
    echo "  ✅ .env 已推送"
    # restart 不会重读 env_file，必须 recreate 容器
    ssh "$SERVER" "cd $SERVER_DIR/tiny-bot-cloud-agent && docker compose --profile https-selfsigned up -d --force-recreate agent"
    echo "  ✅ 容器已重建，新 env 生效"
    MODEL=$(ssh "$SERVER" "docker exec tiny-bot-cloud-agent printenv TB_OPENAI_COMPAT_MODEL 2>/dev/null" || true)
    if [ -n "$MODEL" ]; then
        echo "  📋 容器内 TB_OPENAI_COMPAT_MODEL=$MODEL"
    fi
else
    echo "  ⚠️  本地没有 .env（远端会从 .env.example 复制首次版本）"
fi

# ============================================================
# 5) 健康检查
# ============================================================
echo ""
echo "============================================="
echo "▶ 6/5 健康检查（HTTP + HTTPS）"
echo "============================================="
sleep 3
if curl -sf --max-time 5 "http://$SERVER_HOST:5678/healthz" > /dev/null; then
    echo "  ✅ HTTP /healthz OK"
    curl -s --max-time 5 "http://$SERVER_HOST:5678/healthz"
    echo ""
else
    echo "  ❌ HTTP /healthz 失败"
    echo "  远端日志："
    ssh "$SERVER" "cd $SERVER_DIR/tiny-bot-cloud-agent && docker compose --profile https-selfsigned logs --tail=50"
    exit 1
fi

if curl -skf --max-time 8 "https://$SERVER_HOST/healthz" > /dev/null 2>&1; then
    echo "  ✅ HTTPS /healthz OK"
    curl -sk --max-time 8 "https://$SERVER_HOST/healthz"
    echo ""
else
    echo "  ⚠️  HTTPS 未通：请在云安全组开放 **443**，然后重跑 make deploy"
fi

# ============================================================
# 6) 清理本地 tar
# ============================================================
rm -f "$TAR_FILE" "$CADDY_TAR" "$FILES_TAR"

echo ""
echo "============================================="
echo "🎉 部署完成！"
echo "============================================="
echo "  Demo:    https://$SERVER_HOST/demo/  （手机用这个，首次信任证书）"
echo "  API:     http://$SERVER_HOST:5678"
echo "  健康:    curl -k https://$SERVER_HOST/healthz"
echo "  远端日志: ssh $SERVER 'cd $SERVER_DIR/tiny-bot-cloud-agent && docker compose --profile https-selfsigned logs -f'"
echo "  远端状态: ssh $SERVER 'cd $SERVER_DIR/tiny-bot-cloud-agent && docker compose --profile https-selfsigned ps'"