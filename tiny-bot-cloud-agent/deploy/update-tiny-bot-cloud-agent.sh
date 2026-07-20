#!/bin/bash
# ============================================================
# tiny-bot-cloud-agent 服务部署脚本
# ============================================================
# 用法（两种模式）：
#
# 模式 A：服务器能 pull（docker daemon mirror 配好）
#   ./update-tiny-bot-cloud-agent.sh
#   脚本会：build → down → up → 健康检查
#
# 模式 B：服务器无法 pull（DNS/网络受限场景，国内小 VPS 常见）
#   1) Mac 上 build + save：
#        docker build --platform linux/amd64 -f docker/Dockerfile -t tiny-bot-cloud-agent:dev .
#        docker save tiny-bot-cloud-agent:dev -o tiny-bot-cloud-agent.tar
#        scp tiny-bot-cloud-agent.tar user@host:/path/to/deploy/root/
#   2) 服务器上跑：
#        ./update-tiny-bot-cloud-agent.sh --from-tar /path/to/deploy/root/tiny-bot-cloud-agent.tar
#
# 首次跑前：
#   - 如果还没 .env：脚本会从 .env.example 复制并提醒你 vi 填密钥
#   - 已存在 .env：脚本会跳过覆盖
#   - 已存在 workspace：线上优先保留，包内仅追加缺失文件（记忆/手改人设不被冲掉）
# ============================================================

# 解析参数：--from-tar <path> [--caddy-tar <path>] [--files <path>]
FROM_TAR=""
CADDY_TAR=""
FILES_TAR=""
args=("$@")
i=0
while [ $i -lt ${#args[@]} ]; do
    case "${args[$i]}" in
        --from-tar)  FROM_TAR="${args[$((i+1))]}"; i=$((i+2)) ;;
        --caddy-tar) CADDY_TAR="${args[$((i+1))]}"; i=$((i+2)) ;;
        --files)     FILES_TAR="${args[$((i+1))]}"; i=$((i+2)) ;;
        *) i=$((i+1)) ;;
    esac
done

if [ -n "$FROM_TAR" ] && [ -z "$FROM_TAR" ]; then
    echo "❌ --from-tar 需要指定 tar 路径"
    echo "用法：$0 --from-tar /path/to/tiny-bot-cloud-agent.tar"
    exit 1
fi

set -e

# 部署根目录：脚本所在目录（与 updateAiGeneratorService.sh 同款）
# 假设文件结构（DEPLOY_PATH = 脚本所在目录，即远端 DEPLOY_SERVER_DIR）：
#   $DEPLOY_PATH/update-tiny-bot-cloud-agent.sh  ← 本脚本
#   $DEPLOY_PATH/tiny-bot-cloud-agent.zip       ← 部署包
#   $DEPLOY_PATH/tiny-bot-cloud-agent/          ← 解压后的项目
DEPLOY_PATH="$(cd "$(dirname "$0")" && pwd)"
SERVICE_NAME="tiny-bot-cloud-agent"
SERVICE_DIR="${DEPLOY_PATH}/${SERVICE_NAME}"
ZIP_FILE="${DEPLOY_PATH}/${SERVICE_NAME}.zip"
COMPOSE_FILE="${SERVICE_DIR}/docker-compose.yml"

# ============================================================
# 1) 解压 zip（zip 模式）或 提取 files tar（--from-tar 模式）
# ============================================================
# merge_workspace_prefer_live：线上已有 workspace 优先；包内仅追加缺失文件。
merge_workspace_prefer_live() {
    local live="$1"
    local pack="$2"
    if [ ! -d "$pack" ]; then
        return 0
    fi
    if [ ! -d "$live" ]; then
        mv "$pack" "$live"
        echo "  ✅ workspace 首次落地"
        return 0
    fi
    if command -v rsync >/dev/null 2>&1; then
        rsync -a --ignore-existing "$pack/" "$live/"
    else
        # 无 rsync 时：只复制 live 中不存在的路径
        (
            cd "$pack" || exit 0
            find . -type f -print | while IFS= read -r rel; do
                rel="${rel#./}"
                [ -z "$rel" ] && continue
                if [ ! -e "$live/$rel" ]; then
                    mkdir -p "$(dirname "$live/$rel")"
                    cp -a "$pack/$rel" "$live/$rel"
                fi
            done
        )
    fi
    rm -rf "$pack"
    echo "  ✅ workspace 线上优先合并完成（仅追加缺失文件）"
}

if [ -n "$FILES_TAR" ]; then
    # v1.1.9: --from-tar 模式：先解 files tar 出项目文件
    if [ ! -f "$FILES_TAR" ]; then
        echo "❌ 找不到 files tar $FILES_TAR"
        exit 1
    fi
    echo "▶ 提取项目文件 $FILES_TAR → ${SERVICE_DIR}/"
    mkdir -p "$SERVICE_DIR"

    WS_LIVE="${SERVICE_DIR}/workspace"
    WS_PREV=""
    if [ -d "$WS_LIVE" ]; then
        WS_PREV="${SERVICE_DIR}/workspace.prev.$(date +%Y%m%d%H%M%S)"
        echo "  备份线上 workspace → ${WS_PREV}"
        mv "$WS_LIVE" "$WS_PREV"
    fi

    tar -xzf "$FILES_TAR" -C "$SERVICE_DIR"

    WS_PACK="${SERVICE_DIR}/workspace"
    if [ -n "$WS_PREV" ] && [ -d "$WS_PREV" ]; then
        # 解压出的包内 workspace 挪到临时名，先恢复线上，再只追加缺失
        WS_FROM_PACK="${SERVICE_DIR}/workspace.from-pack.$$"
        if [ -d "$WS_PACK" ]; then
            mv "$WS_PACK" "$WS_FROM_PACK"
        else
            WS_FROM_PACK=""
        fi
        mv "$WS_PREV" "$WS_LIVE"
        if [ -n "$WS_FROM_PACK" ]; then
            merge_workspace_prefer_live "$WS_LIVE" "$WS_FROM_PACK"
        else
            echo "  ✅ 已恢复线上 workspace（包内无 workspace）"
        fi
    else
        echo "  ✅ 项目文件就位（含首次 workspace）"
    fi

    rm -f "$FILES_TAR"
else
    # zip 模式：解压 zip
    if [ ! -f "${ZIP_FILE}" ]; then
        echo "❌ 找不到 ${ZIP_FILE}"
        echo "请先把 ${SERVICE_NAME}.zip 上传到 ${DEPLOY_PATH}/"
        exit 1
    fi
    echo "▶ 解压 ${ZIP_FILE} ..."
    WS_LIVE="${SERVICE_DIR}/workspace"
    WS_PREV=""
    if [ -d "$WS_LIVE" ]; then
        WS_PREV="${SERVICE_DIR}/workspace.prev.$(date +%Y%m%d%H%M%S)"
        echo "  备份线上 workspace → ${WS_PREV}"
        mv "$WS_LIVE" "$WS_PREV"
    fi
    unzip -o "${ZIP_FILE}" -d "${DEPLOY_PATH}"
    if [ -n "$WS_PREV" ] && [ -d "$WS_PREV" ]; then
        WS_FROM_PACK="${SERVICE_DIR}/workspace.from-pack.$$"
        if [ -d "$WS_LIVE" ]; then
            mv "$WS_LIVE" "$WS_FROM_PACK"
        else
            WS_FROM_PACK=""
        fi
        mv "$WS_PREV" "$WS_LIVE"
        if [ -n "$WS_FROM_PACK" ]; then
            merge_workspace_prefer_live "$WS_LIVE" "$WS_FROM_PACK"
        fi
    fi
fi

# 显示解压出来的目录内容
echo ""
echo "▶ 项目目录（前 30 个文件）:"
ls -la "${SERVICE_DIR}" | head -30

# ============================================================
# 2) 进入项目目录
# ============================================================
if [ ! -d "${SERVICE_DIR}" ]; then
    echo "❌ 解压后找不到 ${SERVICE_DIR}"
    exit 1
fi
cd "${SERVICE_DIR}"
echo "  cwd: $(pwd)"

# docker compose profile：默认 https-selfsigned（手机 demo 可 https://IP/demo/）
resolve_compose_profile() {
    COMPOSE_PROFILE="https-selfsigned"
    if [ -f ".env" ]; then
        local val
        val="$(grep -E '^TB_COMPOSE_PROFILE=' .env | tail -1 | cut -d= -f2- | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | tr -d '"' | tr -d "'" || true)"
        if [ -n "$val" ]; then
            COMPOSE_PROFILE="$val"
        fi
    fi
    case "$COMPOSE_PROFILE" in
        none|off|disabled) COMPOSE_PROFILE="" ;;
    esac
}

compose() {
    resolve_compose_profile
    if [ -n "$COMPOSE_PROFILE" ]; then
        docker compose -f "${COMPOSE_FILE}" --profile "${COMPOSE_PROFILE}" "$@"
    else
        docker compose -f "${COMPOSE_FILE}" "$@"
    fi
}

compose_down_all() {
    docker compose -f "${COMPOSE_FILE}" --profile https-selfsigned --profile tunnel --profile https down 2>/dev/null || true
    docker compose -f "${COMPOSE_FILE}" down 2>/dev/null || true
}

# ============================================================
# 3) 检查 docker-compose.yml
# ============================================================
if [ ! -f "${COMPOSE_FILE}" ]; then
    echo "❌ 找不到 ${COMPOSE_FILE}"
    exit 1
fi

# ============================================================
# 4) 检查 .env（首次会从模板复制）
# ============================================================
# v1.1.12: 先等 .env 可能从 Mac 推上来——这次跑是被 make deploy 调用的，
# 真正的 .env 会在步骤 5 推。先在本地目录找，找不到再尝试从 .env.example 复制。
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        echo "⚠️  本地没有 .env，从 .env.example 复制（首次部署）"
        cp .env.example .env
        echo "    ⚠️  请 SSH 上来 vi .env 填密钥，或让 Mac 端 `make deploy-env` 推"
    else
        echo "⚠️  本地没有 .env 也没有 .env.example"
        echo "    期望 Mac 端会推 .env 上来（步骤 5），先不阻断"
    fi
fi

# v1.1.5: 打印当前 .env 路径 + 关键变量状态（让用户知道 docker compose 在读哪个）
if [ -f ".env" ]; then
    echo "📄 .env 路径: $(realpath .env)"
    if grep -qE "^TB_PROVIDER_LLM=openai_compat" .env 2>/dev/null; then
        echo "  ✓ TB_PROVIDER_LLM=openai_compat (会读真 LLM)"
    elif grep -qE "^TB_PROVIDER_LLM=mock" .env 2>/dev/null; then
        echo "  ⚠️  TB_PROVIDER_LLM=mock (不会读真 LLM；改 openai_compat 看真回复)"
    fi
    if grep -qE "^TB_PROVIDER_TTS=minimax" .env 2>/dev/null; then
        echo "  ✓ TB_PROVIDER_TTS=minimax (会读真 TTS)"
    fi
    if [ -n "${TB_OPENAI_COMPAT_API_KEY:-}" ]; then
        echo "  ✓ TB_OPENAI_COMPAT_API_KEY 已在 shell 环境里（deploy 用 env 优先 .env）"
    fi
    echo "============================================================"
fi

# 国内 99 元/年 VPS：daemon.json 没配镜像源 → build 会卡 timeout
# 自动检测 + 自动修（仅当 /etc/docker/daemon.json 不存在或没有 registry-mirrors 时）
if [ -f deploy/daemon.json.cn.example ]; then
    DAEMON_JSON="/etc/docker/daemon.json"
    if [ ! -f "$DAEMON_JSON" ] || ! grep -q "registry-mirrors" "$DAEMON_JSON" 2>/dev/null; then
        # 尝试写（需要 sudo）
        if cp deploy/daemon.json.cn.example "$DAEMON_JSON" 2>/dev/null; then
            echo ""
            echo "▶ 0/5 检测到 /etc/docker/daemon.json 没配镜像源，已自动写入..."
            if command -v systemctl >/dev/null 2>&1; then
                systemctl restart docker 2>/dev/null || true
            fi
            echo "  ✅ daemon.json 已配置，docker 已重启"
        else
            echo ""
            echo "⚠️  /etc/docker/daemon.json 没有镜像源配置"
            echo "   国内环境 build 会卡 timeout，请手动："
            echo "     sudo cp deploy/daemon.json.cn.example /etc/docker/daemon.json"
            echo "     sudo systemctl restart docker"
            echo "   然后重跑本脚本"
            exit 1
        fi
    fi
fi

echo "============================================="
echo "  部署 ${SERVICE_NAME}"
echo "  路径: $(pwd)"
echo "  compose: ${COMPOSE_FILE}"
resolve_compose_profile
echo "  profile: ${COMPOSE_PROFILE:-（无，仅 agent）}"
echo "============================================="

# 5) 构建 / 加载新镜像
echo ""
if [ -n "$FROM_TAR" ]; then
    if [ ! -f "$FROM_TAR" ]; then
        echo "❌ 找不到 $FROM_TAR"
        exit 1
    fi
    echo "▶ 1/5 从 $FROM_TAR 加载 agent 镜像..."
    docker load -i "$FROM_TAR"
    rm -f "$FROM_TAR"
    if [ -n "$CADDY_TAR" ]; then
        if [ ! -f "$CADDY_TAR" ]; then
            echo "❌ 找不到 caddy tar $CADDY_TAR"
            exit 1
        fi
        echo "▶ 1/5 从 $CADDY_TAR 加载 Caddy 镜像..."
        docker load -i "$CADDY_TAR"
        rm -f "$CADDY_TAR"
    fi
    if ! docker image inspect tiny-bot-caddy:2-alpine >/dev/null 2>&1; then
        echo "❌ 缺少 tiny-bot-caddy:2-alpine，请 Mac 端重跑 make deploy"
        exit 1
    fi
else
    echo "▶ 1/5 构建新镜像..."
    docker compose -f ${COMPOSE_FILE} build --no-cache
fi

# 6) 停止旧服务
echo ""
echo "▶ 2/5 停止旧服务..."
compose_down_all

# 7) 创建必要的目录
echo ""
echo "▶ 3/5 创建日志目录..."
mkdir -p "${DEPLOY_PATH}/logs"

# 8) 启动新服务
echo ""
echo "▶ 4/5 启动新服务..."
compose up -d

# 8.5) 自动 seed 默认设备（demo 用，idempotent 重跑安全）
echo ""
echo "▶ 4.5/5 Seed 默认设备..."
if docker ps --format '{{.Names}}' | grep -q "^${SERVICE_NAME}\$"; then
    docker exec "${SERVICE_NAME}" /opt/tiny-bot/seed \
        -device-id tinypal-demo \
        -user-id u-demo \
        -pairing-code DEMO-1234 2>&1 | sed 's/^/  /' || true
fi

# 9) 等待服务启动
echo ""
echo "▶ 5/5 等待服务启动（20s）..."
sleep 20

# 10) 检查服务状态
echo ""
echo "============================================="
echo "  检查服务状态"
echo "============================================="
if compose ps | grep -q "Up"; then
    echo "✅ ${SERVICE_NAME} 部署成功！"
    echo ""
    echo "服务信息:"
    compose ps
    echo ""
    echo "健康检查："
    if curl -sf http://localhost:5678/healthz > /dev/null; then
        echo "  ✅ HTTP  /healthz OK (5678)"
    else
        echo "  ❌ HTTP  /healthz 失败"
    fi
    if curl -skf https://localhost/healthz > /dev/null 2>&1; then
        echo "  ✅ HTTPS /healthz OK (443)"
    elif [ "$COMPOSE_PROFILE" = "https-selfsigned" ]; then
        echo "  ⚠️  HTTPS /healthz 未响应（检查安全组是否开放 443）"
    fi
    echo ""
    echo "运维命令："
    echo "  查看日志:    cd ${SERVICE_DIR} && docker compose --profile ${COMPOSE_PROFILE:-https-selfsigned} -f ${COMPOSE_FILE} logs -f"
    echo "  容器状态:    docker ps | grep ${SERVICE_NAME}"
    echo "  健康检查:    curl http://localhost:5678/healthz"
    echo "  热重载:      cd ${SERVICE_DIR} && docker compose --profile ${COMPOSE_PROFILE:-https-selfsigned} -f ${COMPOSE_FILE} kill -s HUP agent"
    echo "  重启:        cd ${SERVICE_DIR} && docker compose --profile ${COMPOSE_PROFILE:-https-selfsigned} -f ${COMPOSE_FILE} restart"
    echo "  停止:        cd ${SERVICE_DIR} && docker compose --profile ${COMPOSE_PROFILE:-https-selfsigned} -f ${COMPOSE_FILE} down"
    if [ "$COMPOSE_PROFILE" = "tunnel" ]; then
        echo ""
        echo "  手机 Demo HTTPS（Cloudflare 临时地址，重启后会变）:"
        TUNNEL_URL="$(docker compose -f ${COMPOSE_FILE} logs cloudflared 2>&1 | grep -oE 'https://[a-zA-Z0-9-]+\.trycloudflare\.com' | tail -1 || true)"
        if [ -n "$TUNNEL_URL" ]; then
            echo "    ${TUNNEL_URL}/demo/"
        else
            echo "    docker compose -f ${COMPOSE_FILE} logs cloudflared | grep trycloudflare"
        fi
    elif [ "$COMPOSE_PROFILE" = "https-selfsigned" ]; then
        echo ""
        echo "  手机 Demo: https://<服务器公网IP>/demo/ （443，首次在浏览器信任自签证书）"
    fi
else
    echo "❌ 服务启动失败，请检查日志"
    compose logs
    exit 1
fi

echo ""
echo "🎉 部署完成！"
echo "${SERVICE_NAME} 监听: HTTP :5678, HTTPS :443（profile=${COMPOSE_PROFILE:-agent-only}）"
echo "应用环境: \${APP_ENV:-mainnet}"
