# ============================================================
# tiny-bot-cloud-agent - Makefile
# ============================================================

GO           ?= go
BIN_DIR      ?= bin
BIN_NAME     ?= tiny-bot-cloud-agent
BIN          := $(BIN_DIR)/$(BIN_NAME)
PKG          := ./cmd/...
ALL_PKGS     := ./...

# 静态链接、CGo 关闭、符号表裁剪
LDFLAGS      := -s -w
BUILDFLAGS   := -trimpath

.PHONY: help
help: ## 列出本 Makefile 所有目标
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ============================================================
# 构建
# ============================================================

.PHONY: build
build: ## 编译主二进制到 bin/
	mkdir -p $(BIN_DIR)
	$(GO) build $(BUILDFLAGS) -ldflags='$(LDFLAGS)' -o $(BIN) ./cmd/tiny-bot-cloud-agent

.PHONY: build-all
build-all: ## 编译 cmd 下所有可执行（存在才编）
	mkdir -p $(BIN_DIR)
	$(GO) build $(BUILDFLAGS) -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/seed         ./cmd/seed
	@test -d ./cmd/echo-agent && $(GO) build $(BUILDFLAGS) -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/echo-agent ./cmd/echo-agent || true
	@test -d ./cmd/asr-test && $(GO) build $(BUILDFLAGS) -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/asr-test ./cmd/asr-test || true
	$(GO) build $(BUILDFLAGS) -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/$(BIN_NAME)  ./cmd/tiny-bot-cloud-agent
	@test -d ./tools/fake-device && $(GO) build $(BUILDFLAGS) -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/fake-device ./tools/fake-device || true

# ============================================================
# 运行
# ============================================================

.PHONY: run
run: ## 本地启动（自动加载 .env）
	$(GO) run ./cmd/tiny-bot-cloud-agent

.PHONY: run-echo
run-echo: ## 跑 echo-agent 自检
	$(GO) run ./cmd/echo-agent

.PHONY: asr-test
asr-test: ## 直连阿里云 ASR 调试（读 .env + testdata PCM）
	$(GO) run ./cmd/asr-test

.PHONY: llm-test
llm-test: ## 直连 MiniMax LLM 调试（读 .env，验证模型名）
	$(GO) run ./cmd/llm-test

.PHONY: tts-test
tts-test: ## 直连 MiniMax TTS 调试（读 .env，输出 testdata/tts-out.pcm）
	$(GO) run ./cmd/tts-test $(ARGS)

.PHONY: list-voices
list-voices: ## 查询 MiniMax 可用音色（读 .env；ARGS='-q 管家 -type system'）
	$(GO) run ./cmd/list-voices $(ARGS)

# ============================================================
# 测试
# ============================================================

.PHONY: test
test: ## 跑全部单测
	$(GO) test $(ALL_PKGS)

.PHONY: test-race
test-race: ## race detector
	$(GO) test -race $(ALL_PKGS)

.PHONY: test-cover
test-cover: ## 覆盖率
	$(GO) test -coverprofile=coverage.out $(ALL_PKGS) && $(GO) tool cover -func=coverage.out

.PHONY: bench
bench: ## 跑基准
	$(GO) test -bench=. -benchmem ./internal/audio/...

# ============================================================
# 工具
# ============================================================

.PHONY: fake-device
fake-device: build-all ## 跑假 ESP32 客户端
	./$(BIN_DIR)/fake-device -file test/testdata/hello.pcm -url ws://localhost:8080/ws

.PHONY: lint
lint: ## golangci-lint（需先 go install）
	golangci-lint run $(ALL_PKGS)

.PHONY: fmt
fmt: ## 格式化
	gofmt -s -w .

.PHONY: vet
vet: ## 静态检查
	$(GO) vet $(ALL_PKGS)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: clean
clean: ## 清理构建产物
	rm -rf $(BIN_DIR) coverage.out

.PHONY: docker
docker: ## 构建 docker 镜像（linux/amd64，给服务器用；含 Caddy HTTPS 侧车）
	docker build --platform linux/amd64 --provenance=false --sbom=false -f docker/Dockerfile -t $(BIN_NAME):dev .
	docker build --platform linux/amd64 --provenance=false --sbom=false -f docker/Dockerfile.caddy -t tiny-bot-caddy:2-alpine .

# ============================================================
# 部署（Mac → 服务器）
# ============================================================
# 覆盖方式（按优先级）：
#   1) 命令行：make deploy SERVER=user@host
#   2) shell 环境变量：export DEPLOY_SERVER=user@host（或 SERVER=）
#   3) .env 中的 DEPLOY_SERVER / DEPLOY_SERVER_DIR（必填，无硬编码默认）
#
# 从 .env 只读部署相关键（不 include 整文件，避免 API key 特殊字符干扰 make）
ifneq (,$(wildcard .env))
  DEPLOY_SERVER_FROM_ENV := $(shell sed -n 's/^DEPLOY_SERVER=//p' .env | head -1)
  DEPLOY_SERVER_DIR_FROM_ENV := $(shell sed -n 's/^DEPLOY_SERVER_DIR=//p' .env | head -1)
endif
DEPLOY_SERVER ?= $(DEPLOY_SERVER_FROM_ENV)
DEPLOY_SERVER_DIR ?= $(DEPLOY_SERVER_DIR_FROM_ENV)
SERVER       ?= $(DEPLOY_SERVER)
SERVER_DIR   ?= $(DEPLOY_SERVER_DIR)
IMAGE_TAG    ?= dev
TAR_FILE     ?= $(BIN_NAME).tar

# 设备登记（本地 seed / 远端 seed-remote）
DEVICE_ID    ?= tinypal-01
PAIRING_CODE ?= ABCD-1234
DISPLAY_NAME ?= 小主人

.PHONY: require-server
require-server:
	@if [ -z "$(SERVER)" ]; then \
		echo "❌ 未设置 DEPLOY_SERVER / SERVER"; \
		echo "   请在 .env 中设置：DEPLOY_SERVER=user@host"; \
		echo "   或：make <target> SERVER=user@host SERVER_DIR=/path/to/deploy/root"; \
		exit 1; \
	fi
	@if [ -z "$(SERVER_DIR)" ]; then \
		echo "❌ 未设置 DEPLOY_SERVER_DIR / SERVER_DIR"; \
		echo "   请在 .env 中设置：DEPLOY_SERVER_DIR=/path/to/deploy/root"; \
		echo "   或：make <target> SERVER=user@host SERVER_DIR=/path/to/deploy/root"; \
		exit 1; \
	fi

.PHONY: seed
seed: build-all ## 本地 DB 登记设备（仅本机联调；线上无效）
	./$(BIN_DIR)/seed -device-id "$(DEVICE_ID)" -pairing-code "$(PAIRING_CODE)" -display-name "$(DISPLAY_NAME)"
	@echo "✅ 本地已登记 $(DEVICE_ID)。线上请用: make seed-remote DEVICE_ID=$(DEVICE_ID) PAIRING_CODE=$(PAIRING_CODE)"

.PHONY: seed-remote
seed-remote: require-server ## 本机 SSH 到服务器，在 agent 容器内登记设备
	ssh $(SERVER) "cd $(SERVER_DIR)/tiny-bot-cloud-agent && \
		docker compose --profile https-selfsigned exec -T agent \
		/opt/tiny-bot/seed -device-id '$(DEVICE_ID)' -pairing-code '$(PAIRING_CODE)' -display-name '$(DISPLAY_NAME)'"
	@echo ""
	@echo "✅ 远端已登记 $(DEVICE_ID)。请确认固件 config.h 的 TB_DEVICE_ID / TB_PAIRING_CODE 一致，然后复位 ESP32。"

.PHONY: deploy
deploy: require-server docker ## 一键部署到服务器（build + save + scp 代码+env + 远端触发 + 健康检查）
	@if [ ! -f .env ]; then \
		echo ""; \
		echo "==============================================="; \
		echo "  ⚠️  本地没有 .env 文件"; \
		echo "==============================================="; \
		echo "  首次部署：cp .env.example .env && vi .env"; \
		echo "  已有配置：scp .env user@host:.../.env"; \
		echo "  （或 make deploy-env 仅推 .env + 重启，不重 build）"; \
		echo ""; \
		read -p "继续部署（不推 .env）？[y/N] " REPLY; \
		echo ""; \
		if [ "$$REPLY" != "y" ] && [ "$$REPLY" != "Y" ]; then exit 1; fi; \
	fi
	./deploy/deploy.sh $(SERVER) $(SERVER_DIR) $(TAR_FILE) $(BIN_NAME):$(IMAGE_TAG)

.PHONY: deploy-env
deploy-env: require-server ## 只推 .env + 重启容器（你只改了密钥/配置时用，最快）
	@if [ ! -f .env ]; then \
		echo "❌ 本地没有 .env（首次部署用 make deploy，它会自动从 .env.example 复制）"; \
		exit 1; \
	fi
	scp .env $(SERVER):$(SERVER_DIR)/tiny-bot-cloud-agent/.env
	ssh $(SERVER) 'cd $(SERVER_DIR)/tiny-bot-cloud-agent && docker compose --profile https-selfsigned up -d --force-recreate agent'
	@echo ""
	@echo "✅ .env 已推送到服务器并重建容器（reload env_file）"
	@ssh $(SERVER) 'docker exec tiny-bot-cloud-agent printenv TB_OPENAI_COMPAT_MODEL 2>/dev/null || echo "(无法读取容器 env)"'

.PHONY: deploy-logs
deploy-logs: require-server ## 远端 tail 日志
	ssh $(SERVER) 'cd $(SERVER_DIR)/tiny-bot-cloud-agent && docker compose --profile https-selfsigned logs -f --tail=100'

.PHONY: deploy-status
deploy-status: require-server ## 远端查看服务状态
	ssh $(SERVER) 'cd $(SERVER_DIR)/tiny-bot-cloud-agent && docker compose --profile https-selfsigned ps'

.PHONY: deploy-health
deploy-health: require-server ## 本地 curl 健康检查（不需要登录服务器）
	@HOST=$$(echo $(SERVER) | cut -d@ -f2); \
	curl -sf --max-time 5 http://$$HOST:5678/healthz && echo " HTTP OK" || echo " HTTP FAIL"; \
	curl -skf --max-time 8 https://$$HOST/healthz && echo " HTTPS OK" || echo " HTTPS FAIL (检查 443 安全组)"

.PHONY: deploy-restart
deploy-restart: require-server ## 远端重建容器（重读 .env；仅 restart 不会更新 env_file）
	ssh $(SERVER) 'cd $(SERVER_DIR)/tiny-bot-cloud-agent && docker compose --profile https-selfsigned up -d --force-recreate agent'

.PHONY: deploy-stop
deploy-stop: require-server ## 远端停服
	ssh $(SERVER) 'cd $(SERVER_DIR)/tiny-bot-cloud-agent && docker compose --profile https-selfsigned --profile tunnel --profile https down 2>/dev/null; docker compose down 2>/dev/null || true'

# ============================================================
# 本地 demo 前端
# ============================================================
DEMO_PORT ?= 8888

.PHONY: demo
demo: ## 起本地 demo HTTP 服务（python3 -m http.server）
	@echo "打开 http://localhost:$(DEMO_PORT) 浏览器跑 demo"
	@echo "Ctrl+C 退出"
	cd demo && python3 -m http.server $(DEMO_PORT)

.PHONY: demo-open
demo-open: ## 浏览器打开 demo（需先 make demo）
	open http://localhost:$(DEMO_PORT)
