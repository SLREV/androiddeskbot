package bot

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed web/index.html
var indexHTML string

//go:embed web/icons/*.png
var iconsFS embed.FS

// Server hosts the Grok Bot night-desk dashboard and SSE API.
type Server struct {
	cfg      Config
	engine   *Engine
	token    string
	httpSrv  *http.Server
	mu       sync.Mutex
	listener net.Listener
}

// NewServer creates a dashboard Server with a persistent auth token.
func NewServer(cfg Config, engine *Engine) *Server {
	return &Server{cfg: cfg, engine: engine, token: loadOrCreateToken(filepath.Join(cfg.DataDir, ".grokbot-token"))}
}

func loadOrCreateToken(path string) string {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); len(t) >= 16 {
			return t
		}
	}
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	tok := base64.RawURLEncoding.EncodeToString(buf)
	_ = os.WriteFile(path, []byte(tok+"\n"), 0o600)
	return tok
}

// Token returns the dashboard auth token.
func (s *Server) Token() string { return s.token }

// Start binds and serves the dashboard in the background.
func (s *Server) Start() (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/events", s.handleStream)
	mux.HandleFunc("/api/pass", s.handlePass)
	mux.HandleFunc("/api/control", s.handleControl)
	mux.HandleFunc("/api/reset", s.handleReset)
	mux.HandleFunc("/icons/", s.handleIcon)

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.cfg.Bind, s.cfg.Port))
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.listener = ln
	s.httpSrv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	s.mu.Unlock()

	go func() { _ = s.httpSrv.Serve(ln) }()

	host := s.cfg.Bind
	if host == "0.0.0.0" || host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d/?token=%s", host, s.cfg.Port, s.token), nil
}

// Stop gracefully shuts the server down.
func (s *Server) Stop() error {
	s.mu.Lock()
	srv := s.httpSrv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func tokenEqual(a, b string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) isAuthed(r *http.Request) bool {
	if tokenEqual(r.URL.Query().Get("token"), s.token) {
		return true
	}
	if c, err := r.Cookie("grok_token"); err == nil && tokenEqual(c.Value, s.token) {
		return true
	}
	if h := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); tokenEqual(h, s.token) {
		return true
	}
	return false
}

func (s *Server) setCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "grok_token", Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if q := r.URL.Query().Get("token"); tokenEqual(q, s.token) {
		s.setCookie(w)
	}
	if !s.isAuthed(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`<!doctype html><html><body style="background:#0d0d10;color:#eee;font-family:monospace;display:grid;place-items:center;height:100vh"><form action="/" method="get"><h2>Grok Bot</h2><input name="token" placeholder="desk token" style="background:#16161b;border:1px solid #26262e;color:#eee;padding:10px;border-radius:8px"><button style="padding:10px;border-radius:8px;border:0;background:#2dd4a7;color:#03210f;font-weight:700">Connect</button></form></body></html>`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "mode": s.cfg.Mode, "time": time.Now().Format(time.RFC3339)})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.Snapshot())
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	write := func() {
		if b, err := json.Marshal(s.engine.Snapshot()); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
	write()

	ch := s.engine.Subscribe()
	defer s.engine.Unsubscribe(ch)
	for {
		select {
		case <-r.Context().Done():
			return
		case _, open := <-ch:
			if !open {
				return
			}
			write()
		}
	}
}

func (s *Server) handlePass(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.isAuthed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.engine.RunOnce()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.Snapshot())
}

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.isAuthed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Action string `json:"action"`
		Mode   string `json:"mode"`
		On     bool   `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.engine.Control(req.Action, req.Mode, req.On); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.engine.broadcast()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.Snapshot())
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.isAuthed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.engine.Reset()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.engine.Snapshot())
}

// handleIcon serves brand/agent PNGs. Urutan: file di disk (bisa ditukar
// kapan pun tanpa rebuild — <DataDir>/icons/ lalu <exeDir>/icons/), lalu
// fallback ke PNG yang di-embed di binary (tidak pernah blank).
func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/icons/"))
	if name == "" || name == "." || name == "/" || !strings.HasSuffix(strings.ToLower(name), ".png") {
		http.NotFound(w, r)
		return
	}
	for _, dir := range []string{filepath.Join(s.cfg.DataDir, "icons"), filepath.Join(exeDir(), "icons")} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(b)
			return
		}
	}
	if b, err := iconsFS.ReadFile("web/icons/" + name); err == nil {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(b)
		return
	}
	http.NotFound(w, r)
}

// exeDir returns the directory of the running executable ("" if unknown).
func exeDir() string {
	if p, err := os.Executable(); err == nil {
		if rp, err2 := filepath.EvalSymlinks(p); err2 == nil {
			return filepath.Dir(rp)
		}
		return filepath.Dir(p)
	}
	return ""
}
