# Changelog

所有可交付变更都在此追加一行。

## [Unreleased]

### Changed

- **热重载文档对齐代码**：`00-architecture.md` / `CLAUDE.md` 写明 SIGHUP 重载人设四文件与 `workspace/skills/`，**不**重载 `.env`
- **强化 `persona.save_soul` 依从性**：`AGENT.md` 要求改名/性格必须先调工具再口语确认，并允许短版 SOUL Markdown；内置工具描述同步强调「勿只口头答应」
- **默认人设「艾希」**：workspace `IDENTITY`/`SOUL`/`USER`/`AGENT` 从「小朋友小陪」改为全家温热管家；身份口径不主动提 AI；强调实事求是；`llm.LengthDirective` 短答提示改为「等对方」
- **文档补充 `seed` 含义**：README、`cmd/seed` 注释、`02-firmware-integration.md` 写明 seed 是云端预登记设备（入场许可），不是立刻上线；未 seed 会导致 provision 401

### Fixed

- **`force=1` provision 后 WebSocket AUTH_FAIL**：`ReBindDevice` 原先只写 `token_hash`、不把 `status` 置为 `bound`；固件 `TB_PROVISION_FORCE=1` 首次配网走 force 时设备一直 `unbound`，`VerifyToken` 必失败。现改为同时 `status='bound'`。

### Added

- **网页 demo 与硬件设备隔离**：启动时幂等登记 `tinypal-demo` / `u-demo` / `DEMO-1234`，不改其它设备的 token；demo 页提示勿填音箱 Device ID；`make seed-demo-remote` 可在不重启容器时补登记
- **本机受限 Python `code.write` / `code.run`**：写入 `workspace/scratch/<device_id>/*.py`，`python3 -I` 超时执行且不继承 `TB_*` 密钥；复用 `status` 进度文案；配置 `TB_CODE_RUN_TIMEOUT` / `TB_CODE_PYTHON_BIN`；Alpine 镜像安装 `python3`
- **`reminder.set` / `reminder.list`**：记下提醒到 `memory/<device_id>/reminders.md`；明确不会到点响喇叭
- **改名未调 `persona.save_soul` 时补一轮强制 tool 提示**：避免只口头答应
- **情节 Recall 优先走 `memory_index` 候选行**：无命中时回退扫 Markdown
- **单轮多段 `status` 事件流**：WebSocket 下行 `type=status`（`step`/`phase`/`text`/`progress`）；`runToolLoop` 在每个 tool 前后发 `start`/`done|error`（不进 TTS）；固件 OLED 优先显示 `status.text`；配置 `TB_AGENT_TURN_TIMEOUT`（默认 120s）、`TB_AGENT_MAX_TOOL_ROUNDS`（默认 8）
- **`make seed` / `make seed-remote`**：本机可登记设备；`seed-remote` 经 SSH 在服务器 agent 容器内跑 `/opt/tiny-bot/seed`（变量 `DEVICE_ID` / `PAIRING_CODE` / `DISPLAY_NAME`），无需登录服务器手敲
- **`cmd/list-voices`**：调用 MiniMax `/v1/get_voice` 列出账号可用音色；支持 `-type` / `-q` / `-json`；`make list-voices`
- **Demo 免提对话**：能量 VAD 自动开停录音；设置面板可调开始说话阈值 / 静音时长 / 最短说话时长（默认偏保守）；agent 回复播完后再听；保留按住说话
- **Workspace 在线编辑**：`/demo/workspace.html` + `/api/workspace/*`（`TB_WORKSPACE_EDITOR_TOKEN`）；可读写人设四文件、`memory/**/*.md`、`skills/*/SKILL.md`；保存人设触发 `Persona.Reload`
- **部署 workspace 保全**：compose 去掉 `:ro`；entrypoint `chown` workspace；远端解包时线上 workspace 优先，包内仅 `rsync --ignore-existing` 追加缺失文件
- **事实巩固层**：`workspace/memory/<device_id>/facts.md`；`SaveFact` / `ListFacts`；system prompt `## 已知事实`（不受 lookback 限制）；`memory.save` 改写事实层；配置 `TB_AGENT_FACTS_MAX`（默认 80）
- **人设对话改写**：`users.soul_md` + `persona.save_soul` / `persona.save_user`；Turn 用 `resolveSoulMD` / `resolveUserMD` 覆盖 workspace 基线（不写 SOUL.md/USER.md 文件）
- **Tool calling 闭环**：`openai_compat` 解析流式 `tool_calls`；`agent.runToolLoop` 执行 skill / 内置工具后二次 LLM；WebSocket `SendTool` 通知客户端
- **内置记忆工具**：`memory.save` / `memory.recall`（每轮动态注册，无需脚本）
- **per-user 用户画像**：`agent.resolveUserMD` 读取 `users.user_md` 覆盖 `USER.md`；`cmd/seed -user-md` 支持写入
- **对话持久化**：Turn 写入 `conversations`/`messages`；hello 时从 SQLite 恢复 History；断开时 `CloseConversation`
- **记忆索引同步**：`MarkdownStore` 可选注入 indexer，`Record` 时写入 `memory_index`
- **日 Token 限额**：Turn 开头按设备汇总今日 tokens，超限友好退场；流式请求带 `stream_options.include_usage`
- **精细 Interrupt**：session 保存 `turnCancel`，`interrupt` 取消当前 LLM/TTS
- **Skills SIGHUP 热重载**：`skills.Reloader`，与 persona 一并响应 SIGHUP
- **单测**：`openai_compat_tools_test.go`、`agent_tools_test.go`
- **`docs/human-docs/09-internal-packages.md`**：按包说明 `internal/` 下 16 个模块的职责、公开接口、依赖关系、调用链与「改代码该动哪」；补充架构总览之外的包级手册，避免只看 `00-architecture` 时对各包边界不够清晰
- **`docs/human-docs/00-architecture.md`**：新增 cloud-agent 整体架构说明，覆盖模块分层、Turn 时序、SOUL/IDENTITY/AGENT/USER 人设四层、Memory/Skill 机制、v1 已知限制与扩展指南；便于开发者快速理解系统全貌并规划后续改动，无需通读源码
- **`tts.CleanText`**：剥离 LLM 幻觉出的方括号语气乱码（如 `[e~[`），保留正常可读文本与官方圆括号语气词（如 `(laughs)`）；配套 `internal/tts/clean_test.go`
- **`llm.LengthDirective`**：根据用户文本（讲故事/解释/继续）动态追加长答或短答指令
- **`TB_OPENAI_COMPAT_MAX_TOKENS`**：OpenAI 兼容 LLM 请求 `max_tokens`（默认 512）
- **单测**：`internal/llm/length_test.go`
- **多语言文本检测**（`internal/lang`）：ASR 转写后识别普通话（`zh`）、粤语（`yue`）、英语（`en`），供 TTS 音色路由与 LLM 回复语种指令使用；不改造 ASR provider，避免依赖阿里云返回语种字段
- **按语种切换 MiniMax TTS 音色**：`MiniMaxConfig` 新增 `VoiceProfile`（`VoiceZh` / `VoiceYue` / `VoiceEn`），支持 `TB_MINIMAX_TTS_VOICE_ZH|YUE|EN` 与 `TB_MINIMAX_TTS_LANG_BOOST_ZH|YUE|EN`；粤语需配合 `language_boost=Chinese,Yue`（MiniMax 文档要求）
- **`tts.SynthOptions`**：单次合成可覆盖 `voice_id`、`language_boost`、`english_normalization`；MiniMax WebSocket `task_start` 按选项下发参数
- **`llm.LanguageDirective`**：根据检测语种向 system prompt 追加「用普通话/粤语/英语回复」指令，使 LLM 输出与 TTS 音色一致
- **WebSocket `stt` 消息 `lang` 字段**：服务端下发 `zh` / `yue` / `en`，便于 demo 与固件展示识别语种
- **单测**：`internal/lang/detect_test.go`、`internal/agent/agent_lang_test.go`（三语种 turn 路由）、`TestMiniMax_TaskStartLanguageBoost`（验证 WS 帧含 `language_boost`）
- **`cmd/asr-test`**：本地直连阿里云 ASR 的调试 CLI，分步输出 CreateToken 与识别结果，无需跑 WebSocket 全链路；Makefile 新增 `make asr-test`
- **`cmd/llm-test`**：本地直连 MiniMax LLM 的调试 CLI，验证 `TB_OPENAI_COMPAT_MODEL` 与 API Key；Makefile 新增 `make llm-test`
- **`testdata/nls-sample-16k.pcm`**：阿里云官方 16 kHz PCM 样例，供 `asr-test` 与手工验证
- **`internal/aliyunauth/token_test.go`**：CreateToken POP 签名、响应解析、Token 缓存复用单测
- **Demo 线上托管**：agent 在 `/demo/` 提供静态页（Docker 镜像含 `demo/`），`make deploy` 后可直接访问；配置项 `TB_DEMO_ENABLED` / `TB_DEMO_DIR`

### Changed

- **`Agent` 注入完整 `AgentConfig` + `Store`**：记忆 lookback/k、日限额、`MaxResponseChars` 均读配置，不再硬编码 `7, 5`
- **`llm.Chat` 签名**：增加可选 `*StreamResult` 参数（tool_calls / usage 汇总）
- **`server.Deps.Skills`**：由 `*Registry` 改为 `*Reloader`
- **文档**：更新 `00-architecture.md` 附录已知缺口状态与 `09-internal-packages.md` 包说明
- **`docs/human-docs/README.md`**：索引加入 `00-architecture.md` 与 `09-internal-packages.md`
- **`docs/human-docs/00-architecture.md`**：§4 与相关文档表交叉引用 `09-internal-packages.md`；模块表补上 `config` / `logging` / `aliyunauth`
- **`README.md`**：顶部增加架构文档与 `09-internal-packages.md` 链接
- **LLM 回复文本清洗**：Agent turn 在 thinking filter 之后对可见 token 调用 `tts.CleanText`，UI 展示、历史记忆与 TTS 入参保持一致；`synthAndSend` 对清洗后空句跳过合成
- **TTS provider 入口兜底**：MiniMax / Aliyun `Synthesize` 在 `TrimSpace` 后再次 `CleanText`，避免 `tts-test` 等旁路把乱码送进合成
- **`workspace/AGENT.md`**：日常闲聊规则增加「禁止方括号语气标记或 TTS 控制符」，从源头减少 LLM 生成 `[e~[` 类乱码
- **demo 手机麦克风**：HTTP 自动跳转 HTTPS；连接 WS 前预请求麦克风；Chrome `NotAllowedError` 分步引导
- **自适应回复长度**：`workspace/AGENT.md` / `SOUL.md` 分场景规则；`MaxResponseChars` 默认 600 并在 agent 流式输出中软截断
- **空 STT 短路**：转写文本 trim 后为空时跳过 LLM/TTS/记忆写入，仍发 `stt` + `done`，避免空对话消耗配额
- **Agent turn 流程**：STT 后检测输入语种 → 注入 LLM 语种指令 → **按句**检测语种并选对应 MiniMax 音色合成；日志输出 `lang` / `tts route`
- **ASR `Result.Language`**：mock / 阿里云实现由硬编码 `"zh"` 改为对转写文本调用 `lang.Detect`（阿里云仍不解析 API 语种字段，仅文本启发式）
- **`Synthesizer` 接口**：`Synthesize(ctx, text, opts *SynthOptions)`，`opts` 为 `nil` 时各 provider 保持原默认行为
- **`agent.New` 签名**：新增 `config.MiniMaxConfig` 参数，注入各语种音色表；`Sender.SendSTT(text, lang string)` 同步调整
- **配置**：`TB_MINIMAX_TTS_VOICE` 仍作全局 fallback；未设 `TB_MINIMAX_TTS_VOICE_ZH` 时沿用该值；`GET /config` SafeView 增加 `minimax.voices.{zh,yue,en}`
- **demo 前端**：用户 STT 气泡前显示语种标签（如 `[粤语]`）；**手机端响应式**（侧栏抽屉、PTT 触控优化）；Server URL 默认同域；**HTTP 非 localhost 时禁用 PTT 并提示需 HTTPS**
- **make deploy 默认 HTTPS**：远端 compose 默认 `--profile https-selfsigned`；**Caddy 镜像随 tar 离线加载**（`pull_policy: never`，国内 VPS 无需 Docker Hub）
- **`.env.example`**：补充三语种 MiniMax 音色与 `language_boost` 示例注释
- **阿里云 ASR 鉴权改为官方 REST 流程**：AK/SK 通过 `CreateToken` POP API（`nls-meta.{region}.aliyuncs.com`）换取 Token，ASR 请求携带 `X-NLS-Token` Header；v3 签名**仅用于 CreateToken**，不再签名 ASR URL
- **ASR gateway 域名**：默认 `nls-gateway-{region}.aliyuncs.com`（横线分隔，与[官方 REST 文档](https://help.aliyun.com/zh/isi/developer-reference/restful-api-2)一致；v1.1.18 误改为点分隔）
- **ASR query 参数**：去掉 REST 未文档化的 `model`（模型由控制台项目/AppKey 绑定）
- **ASR 响应解析**：兼容扁平 JSON（`{"status":20000000,"result":"..."}`）与嵌套 `header/body` 两种格式（线上实际返回扁平格式）
- **配置**：`TB_ALIYUN_TTS_APP_KEY` 写入独立字段，**不再覆盖** `TB_ALIYUN_ASR_APP_KEY`（避免 ASR 误用 TTS 的 AppKey）
- **WebSocket 错误码**：turn 中 ASR 失败返回 `STT_FAIL`，不再一律误报 `LLM_FAIL`

### Fixed

- **TTS 尾部怪异音效**：LLM 偶发幻觉出 `[e~[ [e~[ ...` 等非法方括号控制符（非 MiniMax 官方 `(laughs)` 格式），原样送入 TTS 会被当作非法字符合成出乱音；现于 Agent 与 provider 双层清洗后剥离
- **ASR 无法打通**：v1.1.18 回退「ASR URL v3 签名」与官方要求不符，导致 `Missing authorization header` / `ACCESS_DENIED`；现按 CreateToken + `X-NLS-Token` 修复，本地 `make asr-test` 与 fake-device 全链路已验证
- **`token.go` 错误实现**：此前调用不存在的 `POST /stream/v1/token` 且解析错误 JSON 字段；现改为 POP `CreateToken`，解析 `Token.Id` / `Token.ExpireTime`
- **LLM 模型名无效**：`TB_OPENAI_COMPAT_MODEL=minimax-3m` 非 MiniMax API 合法 ID（错误码 2013）；默认改为 `MiniMax-M3`，可用 `make llm-test` 本地验证

### Removed

- ASR 请求 query 中的 v3 `Signature` 参数（鉴权职责移至 `X-NLS-Token`）

## 2026-07-04 — v1.1.19: 修复 v3 签名 urlEncode bug（用 PathEscape 错了，应是 QueryEscape）

- **症状**：`internal/aliyunauth/sign.go` 用 `url.PathEscape` 做 URL 编码，**错的**。阿里云 v3 签名规范用的是 `QueryEscape` 语义（`=` 编码 `%3D`，`&` 编码 `%26`，空格编码 `%20`）。PathEscape 不编码 `=` 和 `&`（因为它们在 path 字符集里），导致 query value 含 `=` 或 `&` 时签名错误
- **影响**：当前 ASR query 都是 `appkey`、`format`、`sample_rate`、`model` 这种简单 key（**不含 `=` 或 `&`**），所以**实际生产里没遇到 bug**。但只要 value 含特殊字符，签名就错，会被 Aliyun 拒绝（403 / SignatureDoesNotMatch）
- **修复**：
  - `urlEncode` 简化为 `strings.ReplaceAll(url.QueryEscape(s), "+", "%20")`（用 stdlib + 改空格的 `+` → `%20`）
  - 加两个强校验单测：
    - `TestSign_RealV3Algorithm`：验证 Signature 是 20 字节 SHA1 的 base64（28 字符）
    - `TestSign_ReproducibleHash`：**用 stdlib hmac 独立复算 Signature**，对比 Sign 函数输出（这是最强的本地验证——能复算就说明算法正确）
- **教训**：「我以为 Aliyun v3 签名能跑通就 OK」是错的。**能跑通 ≠ 签名对**。早期 ASR 跑通只是因为 query 都是简单 key。如果用户 query value 加了 `&` 或 `=` 就会突然 403。这种 bug 只靠 smoke test 抓不到，要靠算法级 unit test

## 2026-07-04 — v1.1.18: 放弃 Token 路线，回退 v3 签名（之前诊断全错）

- **症状**：v1.1.17 报 `404 InvalidAction.NotFound`（来自 `nls-meta.cn-shanghai.aliyuncs.com`）
- **根因**（读了 Aliyun SDK 源码后才明白）：
  - 我之前以为的 `nls-meta.<region>.aliyuncs.com/stream/v1/token` 端点**根本不存在**——这是 NLS 文档里抄错或者我理解错了
  - **NLS 流式 ASR 实际上就是 v3 签名 + HTTPS POST**，跟普通 Aliyun OpenAPI 一样
  - "Token 流程"是百炼（Qwen LLM）的鉴权方式，**和 NLS 没关系**
  - 我之前几个版本（v1.1.14-17）一直在错的鉴权方向上试
- **修复**：
  - `internal/asr/aliyun.go` 删 `TokenManager` 字段 + `TokenHost` 配置
  - 改回 `aliyunauth.Sign()` v3 签名（v1.1 的实现）
  - `host()` 用点分隔 `nls-gateway.<region>.aliyuncs.com`（之前是横线，错的）
  - `AliyunConfig` 简化
  - 12 个单测全绿（4 个新加测试 host 解析 + 8 个原有）
- **保留**：`internal/aliyunauth/token.go` 留着以备未来 TTS/其他服务可能用到 Token
- **教训**：之前几个版本都没真调通（404 = 鉴权没配对），但用户每次重新部署都没显式看到 404 错误（因为 `error` 帧把 HTTP 状态掩盖成 "Missing authorization header"），结果一直在错误的鉴权方向上尝试。**下次类似问题**先在服务器抓一次实际 HTTP 请求体看真实错误

## 2026-07-03 — v1.1.17b: Token endpoint 域名格式修正

- **症状**：v1.1.17 报 `dial tcp: lookup nls-meta-cn-shanghai.aliyuncs.com: no such host`
- **根因**：我把域名写成了 `nls-meta-cn-shanghai.aliyuncs.com`（**横线**分隔），实际是 `nls-meta.cn-shanghai.aliyuncs.com`（**点**分隔 `nls-meta` 和 region）
- **修复**：`internal/asr/aliyun.go` `NewAliyun` 改用 `nls-meta.<region>.aliyuncs.com`（点）
- 类似地，ASR gateway 端点之前是 `nls-gateway-<region>.aliyuncs.com`——**也错了**，应该是 `nls-gateway.<region>.aliyuncs.com`（点）。已修。

## 2026-07-03 — v1.1.17: Aliyun ASR 改 Token-based 鉴权（之前一直错）

- **症状**：v1.1.14-16 一直报 `Missing authorization header!`——`APPCODE` 双头/单头都试了不行
- **根因**（看了官方文档才发现）：
  - Aliyun NLS 新协议**不是**直接用 AppKey + Authorization 头
  - 而是 **AppKey 换 Token**：`POST /stream/v1/token` → `{"token":"...","expire_time":...}`
  - 然后用 `Authorization: Bearer <token>` 调 ASR/TTS
  - Token 24h 过期，自动刷新
  - 之前所有 "APPCODE" 尝试都是**完全错的方向**
- **修复**：
  - `internal/aliyunauth/token.go` 新增：`TokenManager`（缓存 + 提前 5min 刷新 + 并发安全）
  - `internal/asr/aliyun.go` 加 `tokens *TokenManager` 字段，`NewAliyun` 自动构造
  - `Transcribe` 流程改成：取 Token → `Authorization: Bearer <token>` 调 ASR
  - `AliyunConfig` 加 `TokenHost` 字段（测试用，指定 nls-meta endpoint）
  - 6 个新单测覆盖：HTTPTest（验证 Bearer token 头）/ TokenCached（同一 token 复用）/ TokenEndpointErr / APIErr / HTTPStatusNot2xx / ParseError
- **关键教训**：之前以为 Aliyun 鉴权是 `APPCODE <appkey>`，实际上**只有老版 sentence/录音文件识别支持**。新版 streaming ASR 全部走 Token 流程
- **同样要改的地方**：`internal/tts/aliyun.go`（v1.1 实现的 CosyVoice TTS）也用了同样的 APPCODE 模式，**同样的 bug**。等 ASR 线上验证后立刻修

## 2026-07-03 — v1.1.16: Aliyun ASR 简化（只发一个 APPCODE 头）

- **症状**：v1.1.15 用 `Header.Add` 同时发 `APPCODE` + `Bearer` 两个 Authorization 头，仍然报 `Missing authorization header!`
- **怀疑**：Aliyun gateway 看到多个 Authorization 头时只读第一个，或者重复头触发了某个 reject
- **修复**：
  - `internal/asr/aliyun.go` 改回只发一个 `APPCODE` 头（最广泛的官方格式）
  - `internal/asr/aliyun_test.go` 验证只设一个 `APPCODE` 头
- **关键调试步骤**：直接 curl Aliyun 验证 AppKey 真的有效（独立于我的代码）
  ```bash
  # 服务器上跑
  curl -X POST \
    "https://nls-gateway-cn-shanghai.aliyuncs.com/stream/v1/asr?appkey=YOUR_APPKEY&format=pcm&sample_rate=16000" \
    -H "Authorization: APPCODE YOUR_APPKEY" \
    -H "Content-Type: application/octet-stream" \
    --data-binary @/dev/urandom
  ```
  期望返回 200 + JSON `{"header":{"status":20000000,"message":"SUCCESS"}}`
  实际返回 ACCESS_DENIED → **AppKey 在阿里云控制台不对**（不是代码问题）

## 2026-07-03 — v1.1.15: Aliyun ASR 双 Authorization 头（用 Add 不是 Set）

- **症状**：v1.1.14 修了 `Authorization: APPCODE` 头但还是报 `Missing authorization header!`
- **根因**：Go 的 `http.Header.Set` **会覆盖**（不是追加）。我之前写：
  ```go
  req.Header.Set("Authorization", "APPCODE xxx")  // 第一次
  req.Header.Set("Authorization", "Bearer xxx")   // 覆盖上面那个
  ```
  实际只发了 `Bearer`，不兼容老网关。同时 `TestAliyun_HTTPTest` 用 `r.Header.Get("Authorization")` 只验证了 1 个值，没发现这个 bug
- **修复**：
  - `internal/asr/aliyun.go` 改用 `req.Header.Add(...)` 同时发两个头
  - `internal/asr/aliyun_test.go` 改用 `r.Header.Values("Authorization")` 验证**两个**值都在
  - 加 `slog.Debug` 输出 `auth_count`（上线后 `TB_LOG_LEVEL=debug` 一眼能看出发了几个头）
- 防御：单测现在能捕捉这种 `Set` vs `Add` 类的 bug

## 2026-07-03 — v1.1.14: Aliyun ASR 补 `Authorization: APPCODE` 头

- **症状**：浏览器 demo 按 PTT 后 server 报 `Gateway:ACCESS_DENIED:Missing authorization header!`
- **根因**：`internal/asr/aliyun.go` 之前只做了 v3 查询串签名（`?Signature=...`），**没设 `Authorization` 头**。`/stream/v1/asr`（一句话识别 REST）实际用 `Authorization: APPCODE <appkey>` 简单鉴权
- **修复**：
  - `internal/asr/aliyun.go` 加 `req.Header.Set("Authorization", "APPCODE "+a.cfg.AppKey)`
  - `internal/asr/aliyun_test.go` `TestAliyun_HTTPTest` 验证 `Authorization: APPCODE test-app-key` 头被设置
- **现状**：v3 签名仍然保留（某些端点需要），叠加 APPCODE 头两端都设——防御性兼容

## 2026-07-03 — v1.1.13: 补 openai_compat LLM 真实现

- **症状**：服务器反复报错 `unknown llm provider "openai_compat"`，浏览器 demo 怎么请求都拿不到真回复
- **根因**：`internal/llm/openai_compat.go` 从 v1.0 起就是占位（v1.1.1 时也只放注释没真写）。`internal/asr/aliyun.go` 和 `internal/tts/aliyun.go` 都写完了，**但 LLM 漏了**
- **修复**：
  - `internal/llm/openai_compat.go` 实现真正的 OpenAI 兼容 chat completions 流式协议
    - POST `/v1/chat/completions`，Bearer auth，stream=true
    - 解析 SSE (`data: {json}\n\n`)，提取 `choices[0].delta.content`
    - finish_reason 非空时触发 `endTurn=true`
    - 支持 `tools` 参数（透传，OpenAI tools 协议）
  - 5 个单测：Name / MissingAPIKey / MissingBaseURL / StreamsSSE（完整流式 + 验证请求格式）/ HTTPError
  - `cmd/tiny-bot-cloud-agent/main.go` `buildLLM` switch 加 `openai_compat` 分支（去掉占位注释）
- **零新增依赖**：用 stdlib `net/http` + `encoding/json` + `bufio`
- **适用**：
  - MiniMax M3（你已经在用）
  - OpenAI GPT-4o-mini / GPT-4o
  - DeepSeek
  - 任何 OpenAI 兼容 API（通过改 `TB_OPENAI_COMPAT_BASE_URL`）

## 2026-07-02 — `make deploy` 两个 bug 修复（v1.1.12）

- **bug 1**：files tar 没包含 `.env.example`（首次部署的模板），导致 server 端 banner 检查 `cp .env.example .env` 失败
- **bug 2**：server 端 "首次部署" 检查顺序错——ssh 触发部署时 `.env` 还没从 Mac 推上来（步骤 5 才推），但 banner 检查就跑了
- 修复：
  - `deploy.sh` files tar 加 `.env.example`（用 `--exclude=.env` 排除真 .env）
  - `update-tiny-bot-cloud-agent.sh` banner 检查软化：本地没 `.env` 时只 warn 不 exit 1（让步骤 5 继续推 .env）

## 2026-07-02 — `make deploy` 顺序 bug：.env scp 在 extract 之前

**症状**：v1.1.9 推项目文件后，scp .env 仍然失败：
```
scp: dest open "$DEPLOY_SERVER_DIR/tiny-bot-cloud-agent/.env": No such file or directory
```
原因：scp `.env` 步骤排在 `ssh 触发部署` 之前，但项目目录是 ssh 触发 extract files tar 时才创建。

**修复**（`deploy/deploy.sh` 重排步骤）：
1. scp tar + files + 部署脚本
2. **ssh 触发部署**（extract files + load image + docker compose up） → 项目目录现在存在
3. **scp .env**（此时项目目录已就绪）→ 成功
4. **ssh docker compose restart** → 容器读新 .env
5. 健康检查

## 2026-07-02 — `make deploy` 现在推项目文件 + 镜像（之前只推镜像）

**症状**：用户删了服务器上 `tiny-bot-cloud-agent/` 文件夹后跑 `make deploy`，scp `.env` 失败：`dest open ... No such file or directory`。`--from-tar` 模式只 load 镜像、**不解压项目文件**（docker-compose.yml / workspace/ / entrypoint.sh），假设项目目录已存在。

**修复**：
- `deploy/deploy.sh` 现在**打两个 tar**：
  - `tiny-bot-cloud-agent.tar`（镜像，docker save 输出）
  - `tiny-bot-cloud-agent-files.tar.gz`（项目文件，含 docker-compose.yml + workspace/ + entrypoint.sh）
  - 两个都 scp 到服务器
- `deploy/update-tiny-bot-cloud-agent.sh` 加 `--files <path>` 参数
  - 跟 `--from-tar <path>` 配套使用
  - 收到 tar 后先 `tar -xzf` 到 `${SERVICE_DIR}/`，再 `docker load` 镜像，再 `docker compose up`
- 测试场景：在 fresh 服务器上跑 `make deploy` → 项目目录自动创建 → 容器正常起来

## 2026-07-02 — `/config` 调试端点 + demo Server Config 面板

- `internal/config/config.go` 加 `(*Config).SafeView()`：返回 map 视图，**不包含任何 secret**（API key、SK、AppKey、Token 全脱敏）
  - 字段对应 .env 同名变量
  - 敏感字段只暴露 `has_api_key: bool` 这种存在性标志
- `internal/server/server.go` 加 `GET /config` handler：返回 `SafeView()` JSON
- `demo/index.html` 加「服务端配置」侧边栏 + 「拉取 /config」按钮 + JSON pretty-print
- 调试场景：浏览器或 `curl http://server:5678/config` 立即看到 server 实际生效的 provider / 凭证是否就绪
- 路由：与 `/healthz` `/readyz` `/metrics` 同列，无需鉴权（因为本身没 secret）

## 2026-07-02 — Mac 本地改 .env 直接 deploy 上去

- `Makefile` 新增 `deploy-env` target：只推 `.env` + 重启容器（你只改了密钥/配置时用，最快）
- `deploy/deploy.sh` 加 scp `.env` 步骤：`make deploy` 也会把 Mac 的 `.env` 推上服务器，**不用 SSH 手动改**
- 流程：
  - 改 Mac 的 `tiny-bot-cloud-agent/.env`
  - `make deploy`  →  build + scp 代码 + scp .env + ssh 触发部署
  - 浏览器刷新就能看到新 provider 生效
- 老的 `update-tiny-bot-cloud-agent.sh` 部署脚本不变（远端行为：unzip 不覆盖 .env，因为 zip 里没有）

## 2026-07-02 — Demo 麦克风优化：复用 stream + 音量浮动条

**症状**：每次按 PTT 浏览器都重新弹麦克风权限框；录音时 PTT 按钮没有音量反馈。

**修复（`demo/index.html`）**：

- **`ensureMicrophone()`**：第一次 PTT 弹权限后，把 `MediaStream` 缓存到 `micStream`。后续录音复用这个 stream，不再调 `getUserMedia` → **不再弹权限**
- **`stopRecording()`** 不再 `track.stop()`（之前每次都关闭 tracks，触发下次重新授权）
- **录音中按钮显示音量浮动条**：用 `AnalyserNode`（fftSize=512）算 RMS，`--level: 0% ~ 100%` 驱动 `conic-gradient` 顺时针填充（绿→黄→红渐变）
- **`onclose`** 时调 `stopLevelMeter()` 清状态

**测试**：全量 go test 仍绿（这次纯 demo 改动）。

## 2026-07-01 — Bug 修复：demo 卡「处理中」死锁

**症状**：浏览器 demo 按住 PTT → 松开 → 状态卡在「处理中」不动，server 日志停在 `ws: hello ok`。

**根因**：`internal/ws/session.go` `handleEnd` 在 audioBuf 为空时**静默 return**（不发 done 也不发 error）。客户端 `stopRecording` 设置「处理中」后只通过收到 `done` 才能切回「已连接」，永远等不到 → 死锁。常见触发：用户短按 PTT < 100ms（MediaRecorder 还没拿到任何 chunk）。

**修复（双端 8 处）**：

服务端（`internal/ws/session.go`）：
- `handleEnd` 即使 buffer 为空也发 `error(ErrProto, "empty audio")` + `done` 给客户端
- `handleAudio` 加 `slog.Debug` 记录每个 chunk（seq/bytes/total_buf）便于以后排查
- `handleEnd` 加 `slog.Info("turn start", "pcm_bytes", len(pcm))` 确认 turn 真正进入处理

客户端（`demo/index.html`）：
- 0 录音守卫：`< MIN_AUDIO_BYTES` 字节直接 reject + 状态回退，不发 audio/end
- 35s watchdog：服务端真挂了 35s 后自动重置状态
- PTT disable：turn 进行中按钮不可点（避免重按排队）
- `safeSend()`：ws.send 错误包装（OPEN 状态误判 / 中断检测）
- 客户端心跳：每 15s 发 ping，30s 没回 pong 主动关
- `onerror` 也更新状态（之前只 log）
- 空格键在 input/textarea 上不触发录音（输入 URL 时不误触）
- `done` / `error` / `onclose` / `onerror` 全部清理 watchdog + 启 PTT

**全量测试通过**（mock / Aliyun / MiniMax 所有 provider 不变）。

## 2026-06-27 — v1.1.1: MiniMax TTS 替换 Aliyun CosyVoice

- `internal/tts/minimax.go` 新增 — WebSocket 流式 TTS（`wss://api.minimaxi.com/ws/v1/t2a_v2`）
  - Bearer Token 鉴权（`Authorization: Bearer <key>`）
  - 协议：JSON 事件流（`task_start` → `task_started` → `task_continue(text)` → audio chunks via `data.audio` 字段（hex 编码 PCM） → `is_final: true` → `task_finish`）
  - 默认模型 `speech-2.8-turbo`，默认音色 `male-qn-qingse`，默认采样率 16 kHz，默认输出 `pcm`
  - 复用 `coder/websocket` 客户端（不新增依赖）
  - 流式实现：goroutine 喂 channel，Read 时优先吐 pending + 监听 ctx.Done
  - 6 个单测：Name / MissingKey / EmptyText / WebSocketConnect / CtxCancel / TaskFinishThenEOF
- `internal/config/config.go` 加 `MiniMaxConfig`（APIKey/Model/Voice/Format/SampleRate/WSURL/Timeout）
  - env 加载：`TB_MINIMAX_API_KEY` / `TB_MINIMAX_TTS_MODEL` / `TB_MINIMAX_TTS_VOICE` 等
  - 便捷：没设 `TB_MINIMAX_API_KEY` 时自动 fallback 用 `TB_OPENAI_COMPAT_API_KEY`（LLM/TTS 共用 key）
  - Validate allowlist 加 `minimax`（TTS 路径）
- `cmd/tiny-bot-cloud-agent/main.go` `buildTTS` switch 加 `minimax` 分支
- 4 个新 config 单测（`TestValidate_MiniMax_OK` / `TestValidate_MiniMax_MissingKey` / `TestLoad_MiniMaxEnvOverlay` / `TestLoad_MiniMax_FallbackFromOpenAIKey`）
- `.env.example` 删 Aliyun TTS 段，加 MiniMax TTS 段；Aliyun 段简化（只剩 ASR）
- **ASR 仍走 Aliyun Paraformer**（MiniMax 独立 ASR endpoint 暂未确认）
- 全量测试通过

## 2026-06-XX — v1.1: Aliyun Paraformer ASR + CosyVoice TTS

新增真接口实现，端到端链路从「mock 占位」升级到「真识别 + 真合成」：

- **`internal/aliyunauth/sign.go`** — 共享的阿里云 v3 HMAC-SHA1 签名工具（ASR + TTS 共用）
  - 7 个单测覆盖：known input、确定性输出、不同 secret/timestamp、空 query、缺凭证
- **`internal/asr/aliyun.go`** — Paraformer 一句话识别 REST 实现
  - 配置：Key / Secret / AppKey / Region / Model（默认 paraformer-realtime-v2）/ Host（测试用）
  - 7 个单测覆盖：Name / 缺凭证 / HTTP 测试 / ctx cancel / 空 PCM / API 错误 / 5xx / JSON 解析错
- **`internal/tts/aliyun.go`** — CosyVoice 短文本语音合成 REST 实现
  - 配置：除 ASR 凭证外，额外支持 Voice / Format / SampleRate（默认 zhitian_emo / pcm / 16000）
  - 6 个单测覆盖：Name / 缺凭证 / 空文本（静音 fallback）/ HTTP 测试 / ctx cancel / 5xx
- **`internal/config/config.go`** — 加 `AliyunConfig` + env overlay (`TB_ALIYUN_*`) + `Validate` allowlist 扩到 `mock|aliyun`
  - 6 个新单测：缺 Key / 缺 AppKey / 全配置 OK / 仅 TTS 用 aliyun / 未知 provider / env 注入
- **`cmd/tiny-bot-cloud-agent/main.go`** — 修 v1.0 bug（`asrImpl := NewMock(""); _ = asrImpl` 是死的，会让 server.go 拿到 nil），加 `buildASR / buildLLM / buildTTS` switch
- **`.env.example`** — 注释展开，把 ASR/TTS 配置段从占位改成可用配置 + 阿里云凭证获取指引
- **依赖**：零新增（用现有 `net/http` + 新加的 `internal/aliyunauth` 共享签名）

### 切换示例

```bash
# .env
TB_PROVIDER_ASR=aliyun
TB_PROVIDER_TTS=aliyun
TB_PROVIDER_LLM=openai_compat
TB_ALIYUN_KEY=<your-ak>
TB_ALIYUN_SECRET=<your-sk>
TB_ALIYUN_ASR_APP_KEY=<your-appkey>
TB_OPENAI_COMPAT_BASE_URL=https://api.minimaxi.com
TB_OPENAI_COMPAT_API_KEY=sk-cp-...
TB_OPENAI_COMPAT_MODEL=minimax-3m

make deploy
# 浏览器 demo 刷新即可看到真识别 + 真声音
```

## 2026-06-26 — CORS middleware（demo 浏览器跨域）

- `internal/server/server.go` 加 `corsMiddleware`：
  - 响应头加 `Access-Control-Allow-Origin: *`
  - 加 `Access-Control-Allow-Methods: GET, POST, OPTIONS`
  - 加 `Access-Control-Allow-Headers: Content-Type, Authorization`
  - OPTIONS 预检 → 204
  - WS 升级不另处理（coder/websocket 默认允许跨域）
- 影响：浏览器 demo（`http://localhost:8888`）可以跨域访问 `http://<server>:5678` 的 HTTP API
- v1 策略 Allow-Origin: *（单 VPS 自部署够用），生产建议改成白名单

## 2026-06-25 — Force Re-Provision（设备 factory reset 支持）

- `internal/auth/auth.go` `Provision` 加 `force bool` 参数
  - `force=false`（默认）：原逻辑，bound 设备拒绝
  - `force=true`：跳过 pairing_code 检查（设备丢 token 后重发）
- `internal/store/devices.go` 加 `ReBindDevice`（覆盖 token_hash，不限制状态机）
- `internal/server/server.go` `/provision` 接受 `?force=1` query param
- `demo/index.html` 默认请求带 `force=1`（调试友好）
- 单测：
  - `TestProvision_ForceReProvision`：验证 force 模式签发新 token + 旧 token 失效
  - `TestMemory_IndexAndSearch`：hardcoded 日期 2026-06-04 改成 today/yesterday（修了 21 天前的 flaky 测试）

## 2026-06-23 — 本地 demo 前端 + entrypoint.sh 修 volume 权限

- **demo/index.html** — 单文件 HTML 调试器，浏览器直接用：
  - HTTP 配网 → WS 连接 → PTT 录音 → 自动播放回复
  - 录音用 MediaRecorder + decodeAudioData → 重采样 16k → Int16 → base64
  - 播放用 AudioContext 串接，无重叠
  - 协议消息全部流到底部日志
- **demo/README.md** — 用法 + 调试技巧
- **Makefile** 加 `make demo` / `make demo-open`（python http server）
- **deploy/entrypoint.sh** + Dockerfile 加 `su-exec`：
  - 修复 named volume mount 时 root:root、tinybot 写不了的 bug
  - entrypoint 以 root chown → su-exec 降权 → exec 二进制
- **deploy** 自动 build：原来 `make deploy` 只 save 没 build，现在 `deploy: docker` 强制依赖

## 2026-06-22 — Mac 端一键部署（少登录服务器）

- 新增 `deploy/deploy.sh`（Mac 端）：一键完成 build → save → scp tar + 部署脚本 → ssh 触发远端 → 健康检查
- 新增 Makefile 部署 targets：
  - `make deploy`（一键部署）
  - `make deploy-logs` / `make deploy-status` / `make deploy-health` / `make deploy-restart` / `make deploy-stop`
- 服务器配置走环境变量（`DEPLOY_SERVER` / `DEPLOY_SERVER_DIR`）；须在本地 `.env` 中填写，仓库无硬编码默认值
- Go 版本对齐：Dockerfile base `golang:1.22-alpine` → `golang:1.25-alpine`；go.mod `go 1.26.1` → `go 1.25.0`

## 2026-06-22 — Tar 模式部署（远端脚本支持 `--from-tar`）

- `deploy/update-tiny-bot-cloud-agent.sh` 加 `--from-tar <path>` 参数：跳过 build，直接 `docker load`，适合服务器无法 pull 的场景
- `deploy/daemon.json.cn.example` 写好 3 个国内 fallback mirror，但如 DNS 不通就只能走 tar 模式

## 2026-06-15 — Dockerfile 改 alpine（解决国内 gcr.io 不可达）

- `docker/Dockerfile` 把运行时 base 从 `gcr.io/distroless/static-debian12:nonroot` 换成 `alpine:3.19`
- 原因：gcr.io 在国内 i/o timeout（之前部署失败就是这个）
- 改动：
  - 加 `apk add ca-certificates wget`（TLS + healthcheck）
  - 加 `adduser tinybot -u 1000`（非 root 运行）
  - `USER tinybot` + 移除 CMD（`update-tiny-bot-cloud-agent.sh` 也不需要 `CMD`）
- 镜像大小：~25 MB（alpine 5MB + 二进制 15MB + ca-certificates 4MB + 运行时 1MB）
- 健康检查仍然用 `wget`（alpine 自带）

## 2026-06-17 — Dockerfile 用 build-arg，让用户配置 corp 镜像

- 进一步发现：所有公开的 Docker Hub 镜像源（Aliyun ACR、DockerProxy、DaoCloud 等）都要求认证了
- 决定：放弃硬编码镜像源，改用 build-arg 让用户传 corp 镜像地址
  - `docker/Dockerfile`：
    ```dockerfile
    ARG GO_BASE=golang:1.22-alpine
    ARG RUNTIME_BASE=alpine:3.19
    FROM ${GO_BASE} AS build
    FROM ${RUNTIME_BASE}
    ```
  - 默认仍是官方名字，让配置好的 docker daemon mirror 自动解析
- 加 `deploy/daemon.json.cn.example`（国内 docker daemon 镜像源配置示例）
- `docker-compose.yml` 加 `build.args` 注释（如何用 corp 镜像源）
- 用法：
  - 方式 A：配置 `/etc/docker/daemon.json`（参考 `deploy/daemon.json.cn.example`）后用默认 base
  - 方式 B：`docker compose build --build-arg GO_BASE=<corp>/library/golang:1.22-alpine ...`

## 2026-06-16 — Dockerfile 改用 ACR 公共镜像（解决国内 docker.io 也不可达）

- 进一步发现：`docker.io` 整个都被封了，`golang:1.22-alpine` 也拉不下来
- 改用阿里云 ACR 公共命名空间 `registry.cn-hangzhou.aliyuncs.com/library/...`
  - build 阶段：`registry.cn-hangzhou.aliyuncs.com/library/golang:1.22-alpine`
  - runtime 阶段：`registry.cn-hangzhou.aliyuncs.com/library/alpine:3.19`
- 不需要任何 docker daemon 镜像配置
- 阿里云公共镜像：免登录、走国内 CDN、速度 < 1s

## 2026-06-14 — 部署脚本补 `unzip` + `cd` 步骤

- `deploy/update-tiny-bot-cloud-agent.sh` 修：
  - 加 `unzip -o tiny-bot-cloud-agent.zip`（之前要求用户手动 unzip 后再跑，不符合 `updateAiGeneratorService.sh` 的「一键部署」模式）
  - 加 `cd "${SERVICE_DIR}"`（之前假设用户在项目根目录跑，违反模式）
  - `DEPLOY_PATH` 改成 `$(cd $(dirname $0) && pwd)`（脚本所在目录 = 部署根目录）
  - 首次部署时若没 `.env` 会从 `.env.example` 复制并提示 `vi .env` 填密钥

## 2026-06-12 — 单一配置源（删除 `config.yaml`）

- **删除 `config.yaml`**，所有配置统一走 `.env`（一份文件管所有）
- 调整：
  - `.env` / `.env.example` 加 `TB_WS_PATH` / `TB_PROVIDER_*` / `TB_AGENT_*` / `TB_HEALTHZ_ENABLED` / `TB_PPROF_ENABLED` / `TB_MAX_FRAME_BYTES` / `TB_WS_PING_INTERVAL` / `TB_WS_IDLE_TIMEOUT` 全部字段
  - `internal/config/config.go` 砍掉 YAML 解析；`Load(_ string)` 签名保留但参数废弃
  - `cmd/tiny-bot-cloud-agent/main.go` 去掉 `-config` flag
  - `cmd/seed/main.go` 去掉 `-config` flag
  - `Makefile` 去掉 `CONFIG` 变量，`make run` 不再传 flag
  - `deploy/tiny-bot-cloud-agent.service` `ExecStart` 不再传 `-config`
  - `docker/Dockerfile` 不再 `COPY config.yaml` / `CMD -config`
  - `docker-compose.yml` 删掉注释里的 config.yaml 挂载
  - `tiny-bot-firmware/README.md` / `CLAUDE.md` / `include/config.h.example` 引用同步
  - `docs/human-docs/01-database.md` / `02-firmware-integration.md` 引用同步
- 依赖不变：`go.yaml.in/yaml/v3` 仍被 `internal/skills/loader.go` 用于解析 SKILL.md frontmatter

## 2026-06-12 — .env 自动加载 + 配置优先级文档

- `internal/config/config.go` 新增 `loadDotEnv()`：进程启动时读 `./.env` 并注入到 `os.Environ`，已存在的 env 不会被覆盖（防被注入劫持）
- 行为：本地 `./bin/tiny-bot-cloud-agent` 跟 `docker compose up` 行为一致（都吃 .env）
- `internal/config/config_test.go` 加 2 个测试：`TestLoadDotEnv`（基础 + 注释 + 引号 + 空值 + 非法行）、`TestLoadDotEnv_DoesNotOverrideExistingEnv`
- `CLAUDE.md` 加「配置：两个文件怎么配合？」一节，明确优先级 + 加载顺序 + 典型场景
- 配套把 `.env` 和 `.env.example` 的 `TB_AGENT_ADDR=:8080` 改成 `:5678`（之前漏改）

## 2026-06-10 — 默认端口改 5678

- 把所有 `8080` 统一改为 `5678`（config 默认、docker-compose 端口映射、healthcheck、部署脚本、fake-device 默认 URL、文档与 OpenAPI、固件 config.h.example）
- 影响：本地默认端口、docker-compose `-p 5678:5678`、所有 README 里的 `curl`/`ws://` 示例

## 2026-06-08 — docker-compose 部署支持

- 新增 `docker-compose.yml`（端口映射、`.env` 自动加载、`workspace` 只读挂载、命名卷持久化 SQLite、healthcheck、日志轮转、CPU/内存 limits）
- 新增 `deploy/update-tiny-bot-cloud-agent.sh`（与项目内 `updateAiGeneratorService.sh` 同款模式：build → down → up → 健康检查；可直接复制到服务器跑）
- `.env.example` 增补 `TB_PROVIDER_*` 与 `TB_FAKE_ASR_TEXT`
- README 部署章节把 docker-compose 提升为"方式 A（推荐）"，systemd 降为"方式 B（裸机）"
- CLAUDE.md 目录结构同步

## 2026-06-08 — v0.1 端到端可用

10 个阶段全部完成，端到端链路（`/provision` → WebSocket `hello`/`audio`/`end` → STT → LLM 流式 → 句子聚合 → TTS → `done`）实测通过。

实施内容：

- **Phase 0 引导**：`go mod init`、目录骨架、最小 main、Makefile、CLAUDE.md、`.gitignore`、`config.yaml`、`.env.example`、workspace 占位
- **Phase 1 配置 + 日志**：`internal/config`（YAML + env 覆盖 + 严格校验）、`internal/logging`（`log/slog` JSON/text + ctx logger）
- **Phase 2 存储**：`internal/store`（`modernc.org/sqlite` 纯 Go 驱动、WAL、读写连接分离）、schema v1、迁移 runner、用户/设备/会话/消息/记忆索引 CRUD、`cmd/seed`
- **Phase 3 人设 + 记忆**：`internal/persona`（SOUL/IDENTITY/AGENT/USER 加载、Reloader 支持 SIGHUP）、`internal/memory`（Markdown 按 device/天分文件、关键词+时间衰减打分、Top-K 召回）
- **Phase 4 音频**：`internal/audio`（PCM16 base64 编解码、Header 元信息、流式句子聚合器，基准 22 ns/token）
- **Phase 5 provider 接口**：`internal/asr`、`internal/tts`、`internal/llm` 三个接口 + Mock 实现；`llm.BuildSystemPrompt` 组合 SOUL→IDENTITY→AGENT→USER→记忆→技能；`cmd/echo-agent` 跑通文本→完整 pipeline
- **Phase 6 技能**：`internal/skills`（SKILL.md frontmatter 解析、registry、tool_def 转换、runner exec 脚本 5s 超时 8KB 截断）、`workspace/skills/example-weather` 示例
- **Phase 7 服务**：`internal/ws`（`coder/websocket`、协议、session 状态机）、`internal/auth`（pairing code + 长期 token + SHA-256 + 时序安全比对）、`internal/agent`（turn 循环 + 工具路由 + history ring buffer）、`internal/server`（HTTP 路由）、`tools/fake-device`（模拟 ESP32）
- **Phase 8 端点**：`/healthz`、`/readyz`、`/metrics`、`/provision`、`/ws` 全部就绪
- **Phase 9 部署**：`deploy/tiny-bot-cloud-agent.service`（systemd unit，MemoryMax=800M）、`docker/Dockerfile`（distroless static，~20MB）
- **Phase 10 抛光**：SIGHUP 热重载（已实现）、README quickstart、`test/integration_test.go` 端到端测试

## 2026-06-05 — Phase 0: Bootstrap

- 初始化 Go 模块 `github.com/wisdomoasis/tiny-bot-cloud-agent`
- 创建目录骨架（cmd / internal / tools / workspace / deploy / docker / test）
- 最小 `cmd/tiny-bot-cloud-agent/main.go`（输出 hello）
- Makefile（build / run / test / fake-device / lint / tidy）
- `config.yaml` 默认配置、`CLAUDE.md` 项目规范、`.gitignore`、`.env.example`、本 CHANGELOG
