// Package server 暴露 HTTP 路由：/ws、/healthz、/readyz、/metrics、/provision、/config、/api/workspace/*、/demo。
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/agent"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/auth"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/skills"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/ws"
)

// Deps 依赖注入。
type Deps struct {
	Cfg     *config.Config
	Store   *store.Store
	Auth    *auth.Service
	Persona *persona.Reloader
	Memory  memory.Store
	Skills  *skills.Reloader
	History *agent.HistoryRegistry
	ASR     asr.Transcriber
	LLM     llm.Chat
	TTS     tts.Synthesizer
}

// Server 顶层 HTTP 服务。
type Server struct {
	deps    Deps
	mux     *http.ServeMux
	httpSrv *http.Server
	started time.Time
	ready   atomic.Bool
}

// New 构造一个 server。
func New(deps Deps) *Server {
	s := &Server{deps: deps, mux: http.NewServeMux(), started: time.Now()}
	s.routes()
	s.httpSrv = &http.Server{
		Addr:              deps.Cfg.Server.Addr,
		Handler:           corsMiddleware(s.mux),  // 包一层 CORS，让浏览器 demo 能跨域调
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/ws", s.handleWS)
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/readyz", s.handleReady)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/provision", s.handleProvision)
	s.mux.HandleFunc("/config", s.handleConfig) // v1.1.8: 调试用配置视图（无 secret）
	s.registerWorkspaceEditorRoutes()
	s.registerDemoRoutes()
}

func (s *Server) registerDemoRoutes() {
	dir := s.deps.Cfg.Server.ResolveDemoDir()
	if dir == "" {
		if s.deps.Cfg.Server.DemoEnabled {
			slog.Info("demo disabled: directory not found")
		}
		return
	}
	fs := http.FileServer(http.Dir(dir))
	s.mux.Handle("/demo/", http.StripPrefix("/demo/", fs))
	s.mux.HandleFunc("/demo", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/demo" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/demo/", http.StatusFound)
	})
	slog.Info("demo static files", "dir", dir, "url", "/demo/")
}

// corsMiddleware 给 HTTP 响应加 CORS 头（让浏览器 demo 跨域调用）。
//
// v1 策略：允许所有 origin（`Access-Control-Allow-Origin: *`）。
// 适合：
//   - 浏览器 demo（demo/index.html）跨域调 HTTP API
//   - 跨域场景下的 GET/POST/OPTIONS 预检
//
// 生产环境建议改成白名单（只允许自己的域）。
//
// WebSocket 升级是另一套机制，浏览器默认允许跨域 WS（除非服务器校验 Origin 头）。
// coder/websocket 默认不校验 Origin，因此 WS 在这里不需要额外处理。
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24h，预检缓存
		// OPTIONS 预检直接 204
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ListenAndServe 阻塞。返回 ErrServerClosed 时正常。
func (s *Server) ListenAndServe() error {
	slog.Info("http listen", "addr", s.deps.Cfg.Server.Addr)
	s.ready.Store(true)
	if err := s.httpSrv.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

// Shutdown 优雅退出。
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// writeJSON 工具：把 v 序列化为 JSON 写到 w。
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"ok":     true,
		"uptime": int(time.Since(s.started).Seconds()),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ready": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ready": true})
}

// v1.1.8: handleConfig 返回"安全"的配置视图（不包含任何 secret）。
// 用途：让 demo 浏览器或 curl 能直接看到当前服务器在用什么 provider。
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Cfg.SafeView())
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"uptime_seconds": int(time.Since(s.started).Seconds()),
		"go_version":     "1.22+",
		"providers": map[string]string{
			"asr": s.deps.ASR.Name(),
			"llm": s.deps.LLM.Name(),
			"tts": s.deps.TTS.Name(),
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	deviceID := r.URL.Query().Get("device_id")
	code := r.URL.Query().Get("code")
	force := r.URL.Query().Get("force") == "1" || r.URL.Query().Get("force") == "true"
	if deviceID == "" || code == "" {
		http.Error(w, "device_id and code required", http.StatusBadRequest)
		return
	}
	tok, err := s.deps.Auth.Provision(r.Context(), deviceID, code, force)
	if err != nil {
		slog.Warn("provision failed", "device_id", deviceID, "err", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id": deviceID,
		"token":     tok,
		"hint":      "store this token and use it in WebSocket hello",
	})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.Upgrade(w, r, s.deps.Cfg.Server.IdleTimeout)
	if err != nil {
		slog.Warn("ws upgrade", "err", err)
		return
	}
	ag := agent.New(
		s.deps.Persona, s.deps.Memory, s.deps.Skills,
		s.deps.History, s.deps.Store, s.deps.ASR, s.deps.LLM, s.deps.TTS,
		s.deps.Cfg.MiniMax,
		s.deps.Cfg.Agent,
	)
	ag.WorkspaceRoot = s.deps.Cfg.Storage.WorkspaceRoot
	sess := ws.NewSession(conn)
	sess.TurnTimeout = s.deps.Cfg.Agent.TurnTimeout
	ag.Bind(sess, s.deps.Auth.VerifyToken)
	slog.Info("ws connected", "remote", r.RemoteAddr)
	sess.Run()
	slog.Info("ws disconnected")
}
