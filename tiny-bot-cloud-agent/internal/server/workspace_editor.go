package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxWorkspaceFileBytes = 1 << 20 // 1 MiB

var personaRootFiles = map[string]struct{}{
	"SOUL.md":     {},
	"IDENTITY.md": {},
	"AGENT.md":    {},
	"USER.md":     {},
}

func (s *Server) registerWorkspaceEditorRoutes() {
	s.mux.HandleFunc("/api/workspace/files", s.handleWorkspaceFiles)
	s.mux.HandleFunc("/api/workspace/file", s.handleWorkspaceFile)
}

func (s *Server) workspaceEditorEnabled() bool {
	return strings.TrimSpace(s.deps.Cfg.Server.WorkspaceEditorToken) != ""
}

func (s *Server) requireWorkspaceEditorAuth(w http.ResponseWriter, r *http.Request) bool {
	if !s.workspaceEditorEnabled() {
		http.NotFound(w, r)
		return false
	}
	want := strings.TrimSpace(s.deps.Cfg.Server.WorkspaceEditorToken)
	got := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(got, prefix) || strings.TrimSpace(got[len(prefix):]) != want {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) handleWorkspaceFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireWorkspaceEditorAuth(w, r) {
		return
	}
	root := s.deps.Cfg.Storage.WorkspaceRoot
	files, err := listWorkspaceMarkdown(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (s *Server) handleWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireWorkspaceEditorAuth(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rel := strings.TrimSpace(r.URL.Query().Get("path"))
		abs, err := resolveWorkspaceMarkdownPath(s.deps.Cfg.Storage.WorkspaceRoot, rel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		raw, err := os.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": filepath.ToSlash(rel), "content": string(raw)})
	case http.MethodPut:
		var body struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		defer r.Body.Close()
		limited := io.LimitReader(r.Body, maxWorkspaceFileBytes+1024)
		if err := json.NewDecoder(limited).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if len(body.Content) > maxWorkspaceFileBytes {
			http.Error(w, "content too large", http.StatusRequestEntityTooLarge)
			return
		}
		abs, err := resolveWorkspaceMarkdownPath(s.deps.Cfg.Storage.WorkspaceRoot, body.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmp := abs + ".tmp"
		if err := os.WriteFile(tmp, []byte(body.Content), 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmp, abs); err != nil {
			_ = os.Remove(tmp)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rel := filepath.ToSlash(strings.TrimSpace(body.Path))
		if isPersonaRootFile(rel) && s.deps.Persona != nil {
			if err := s.deps.Persona.Reload(); err != nil {
				slog.Warn("persona reload after workspace edit", "path", rel, "err", err)
			} else {
				slog.Info("persona reloaded after workspace edit", "path", rel)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": rel})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// resolveWorkspaceMarkdownPath 校验相对路径在白名单内，返回绝对路径。
func resolveWorkspaceMarkdownPath(workspaceRoot, rel string) (string, error) {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return "", fmt.Errorf("path required")
	}
	if strings.Contains(rel, "..") || strings.Contains(rel, "\\") {
		return "", fmt.Errorf("invalid path")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".md") {
		return "", fmt.Errorf("only .md allowed")
	}
	if !isAllowedWorkspaceMarkdown(rel) {
		return "", fmt.Errorf("path not allowed")
	}
	rootAbs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(rootAbs, filepath.FromSlash(rel))
	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	relToRoot, err := filepath.Rel(rootAbs, abs)
	if err != nil || strings.HasPrefix(relToRoot, "..") {
		return "", fmt.Errorf("path escapes workspace")
	}
	return abs, nil
}

func isPersonaRootFile(rel string) bool {
	_, ok := personaRootFiles[filepath.ToSlash(rel)]
	return ok
}

func isAllowedWorkspaceMarkdown(rel string) bool {
	rel = filepath.ToSlash(rel)
	if isPersonaRootFile(rel) {
		return true
	}
	if strings.HasPrefix(rel, "memory/") && strings.Count(rel, "/") >= 1 {
		return true
	}
	// skills/<name>/SKILL.md
	parts := strings.Split(rel, "/")
	if len(parts) == 3 && parts[0] == "skills" && parts[1] != "" && parts[2] == "SKILL.md" {
		return true
	}
	return false
}

func listWorkspaceMarkdown(workspaceRoot string) ([]string, error) {
	rootAbs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, err
	}
	var out []string
	for name := range personaRootFiles {
		p := filepath.Join(rootAbs, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, name)
		}
	}
	memRoot := filepath.Join(rootAbs, "memory")
	_ = filepath.Walk(memRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if isAllowedWorkspaceMarkdown(rel) {
			out = append(out, rel)
		}
		return nil
	})
	skillsRoot := filepath.Join(rootAbs, "skills")
	entries, _ := os.ReadDir(skillsRoot)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("skills", e.Name(), "SKILL.md"))
		p := filepath.Join(rootAbs, filepath.FromSlash(rel))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out, nil
}
