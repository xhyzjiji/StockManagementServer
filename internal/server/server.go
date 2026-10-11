// Package server 提供 JSON API、受鉴权保护的图片访问和前端静态文件服务。
// Package server serves the JSON API, auth-protected image files and the frontend static files.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"private/construction_warehouse/internal/auth"
	"private/construction_warehouse/internal/config"
	"private/construction_warehouse/internal/model"
	"private/construction_warehouse/internal/store"
)

type Server struct {
	cfg     *config.Config
	store   *store.Store
	auth    *auth.Service
	limiter *loginLimiter
}

func New(cfg *config.Config, st *store.Store, au *auth.Service) *Server {
	return &Server{cfg: cfg, store: st, auth: au, limiter: newLoginLimiter(10, 10*time.Minute)}
}

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxToken
)

func userOf(r *http.Request) *model.User { return r.Context().Value(ctxUser).(*model.User) }
func uid(r *http.Request) int64          { return userOf(r).ID }
func tokenOf(r *http.Request) string     { return r.Context().Value(ctxToken).(string) }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	a := s.requireAuth

	mux.HandleFunc("GET /api/auth/config", s.handleAuthConfig)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.Handle("GET /api/auth/me", a(s.handleMe))
	mux.Handle("POST /api/auth/password", a(s.handleChangePassword))

	mux.Handle("GET /api/warehouses", a(s.handleListWarehouses))
	mux.Handle("POST /api/warehouses", a(s.handleCreateWarehouse))
	mux.Handle("GET /api/warehouses/{id}", a(s.handleGetWarehouse))
	mux.Handle("PUT /api/warehouses/{id}", a(s.handleUpdateWarehouse))
	mux.Handle("DELETE /api/warehouses/{id}", a(s.handleDeleteWarehouse))
	mux.Handle("GET /api/warehouses/{id}/export", a(s.handleExport))
	mux.Handle("GET /api/recycle/warehouses", a(s.handleListDeletedWarehouses))
	mux.Handle("POST /api/recycle/warehouses/{id}/restore", a(s.handleRestoreWarehouse))
	mux.Handle("DELETE /api/recycle/warehouses/{id}", a(s.handleDestroyWarehouse))
	mux.Handle("POST /api/import/prepare", a(s.handleImportPrepare))
	mux.Handle("POST /api/import/commit", a(s.handleImportCommit))

	mux.Handle("GET /api/warehouses/{id}/categories", a(s.handleListCategories))
	mux.Handle("POST /api/warehouses/{id}/categories", a(s.handleCreateCategory))
	mux.Handle("GET /api/categories/{id}", a(s.handleGetCategory))
	mux.Handle("PUT /api/categories/{id}", a(s.handleRenameCategory))
	mux.Handle("DELETE /api/categories/{id}", a(s.handleDeleteCategory))

	mux.Handle("GET /api/warehouses/{id}/attrs", a(s.handleListAttrs))
	mux.Handle("PUT /api/warehouses/{id}/attrs", a(s.handleSaveAttrs))

	mux.Handle("GET /api/warehouses/{id}/items", a(s.handleListItems))
	mux.Handle("POST /api/warehouses/{id}/items/delete", a(s.handleDeleteItems))
	mux.Handle("GET /api/warehouses/{id}/recycle/items", a(s.handleListDeletedItems))
	mux.Handle("POST /api/recycle/items/{id}/restore", a(s.handleRestoreItem))
	mux.Handle("DELETE /api/recycle/items/{id}", a(s.handleDestroyItem))
	mux.Handle("POST /api/items", a(s.handleCreateItem))
	mux.Handle("GET /api/items/{id}", a(s.handleGetItem))
	mux.Handle("PUT /api/items/{id}", a(s.handleUpdateItem))

	mux.Handle("POST /api/items/{id}/stock", a(s.handleApplyStock))
	mux.Handle("GET /api/items/{id}/records", a(s.handleItemRecords))
	mux.Handle("GET /api/items/{id}/prices", a(s.handleItemPrices))
	mux.Handle("GET /api/warehouses/{id}/records", a(s.handleWarehouseRecords))
	mux.Handle("GET /api/warehouses/{id}/stats", a(s.handleStats))
	mux.Handle("GET /api/warehouses/{id}/stats/tags", a(s.handleTagStats))

	mux.Handle("POST /api/uploads/image", a(s.handleUploadImage))
	mux.Handle("GET /files/{path...}", a(s.handleFile))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "接口不存在")
	})
	if s.cfg.Server.WebDir != "" {
		mux.Handle("/", staticHandler(s.cfg.Server.WebDir))
	}

	return s.recoverer(s.logger(mux))
}

// staticHandler 托管前端构建产物：带 hash 的 /assets/ 长期缓存，index.html 禁止缓存；
// 不存在的路径回退到 index.html，以支持前端 history 路由。
// staticHandler serves the frontend build: hashed /assets/ are cached long-term and index.html never;
// unknown paths fall back to index.html so frontend history routes work.
func staticHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		p := path.Clean("/" + r.URL.Path)
		if p != "/" {
			if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "/assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fs.ServeHTTP(w, r)
				return
			}
			// 缺失的构建资源直接 404，不回退到 index.html，避免旧页面拿到 HTML 当脚本执行
			// Missing build assets return 404 instead of index.html, so stale pages never run HTML as a script.
			if strings.HasPrefix(p, "/assets/") {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}

// ---------------- 中间件 / Middleware ----------------

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *Server) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
		}
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				log.Printf("panic: %v\n%s", p, debug.Stack())
				writeError(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAuth(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if c, err := r.Cookie(s.cfg.Auth.CookieName); err == nil {
			token = c.Value
		}
		u, err := s.auth.Authenticate(r.Context(), token)
		if err != nil {
			if !errors.Is(err, auth.ErrUnauthorized) {
				log.Printf("authenticate: %v", err)
			}
			s.clearCookie(w)
			writeError(w, http.StatusUnauthorized, auth.ErrUnauthorized.Error())
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, u)
		ctx = context.WithValue(ctx, ctxToken, token)
		h(w, r.WithContext(ctx))
	})
}

// ---------------- Cookie ----------------

func (s *Server) setTokenCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Auth.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Round(time.Second).Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.Auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Auth.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.Auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ---------------- 请求 / 响应工具 / Request & response helpers ----------------

type apiError struct {
	Error string `json:"error"`
	// 出库超量时返回当前库存
	// Current stock, returned when an outbound exceeds it.
	Available *float64 `json:"available,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func writeOK(w http.ResponseWriter) { writeJSON(w, http.StatusOK, map[string]bool{"ok": true}) }

// fail 把业务错误映射为 HTTP 状态码；未知错误记录日志并返回 500。
// fail maps domain errors to HTTP status codes; unknown errors are logged and returned as 500.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *model.ValidationError
	var ie *model.InsufficientStockError
	var mbe *http.MaxBytesError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, ve.Msg)
	case errors.As(err, &ie):
		avail := ie.Available
		writeJSON(w, http.StatusConflict, apiError{Error: "库存不足", Available: &avail})
	case errors.As(err, &mbe):
		writeError(w, http.StatusRequestEntityTooLarge, "上传内容过大")
	case errors.Is(err, model.ErrNotFound):
		writeError(w, http.StatusNotFound, model.ErrNotFound.Error())
	case errors.Is(err, model.ErrForbidden):
		writeError(w, http.StatusForbidden, model.ErrForbidden.Error())
	case errors.Is(err, store.ErrUserExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, auth.ErrBadCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, context.Canceled):
		// 客户端已断开 / client went away
	default:
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return err
		}
		if errors.Is(err, io.EOF) {
			return model.Invalid("请求体为空")
		}
		return model.Invalid("请求格式错误")
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, model.ErrNotFound
	}
	return id, nil
}

func queryInt(r *http.Request, key string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(key)); err == nil {
		return v
	}
	return def
}

// clientIP 返回客户端 IP；开启 trust_proxy 时取反向代理设置的 X-Real-IP（不可直接暴露后端端口，否则可被伪造）。
// clientIP returns the client IP; with trust_proxy it uses X-Real-IP from the reverse proxy
// (the backend port must not be exposed directly, or the header can be spoofed).
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.Server.TrustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(ip) != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func cleanExt(name string) string { return strings.ToLower(filepath.Ext(name)) }

// ---------------- 登录限流 / Login rate limiting ----------------

// loginLimiter 按客户端 IP 限制窗口内的失败登录次数，防止暴力破解。
// loginLimiter caps failed logins per client IP within a window to slow down brute force.
type loginLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	fails  map[string][]time.Time
}

func newLoginLimiter(max int, window time.Duration) *loginLimiter {
	return &loginLimiter{max: max, window: window, fails: map[string][]time.Time{}}
}

func (l *loginLimiter) recent(key string, now time.Time) []time.Time {
	var kept []time.Time
	for _, t := range l.fails[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key)
	} else {
		l.fails[key] = kept
	}
	return kept
}

func (l *loginLimiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key, time.Now())) >= l.max
}

func (l *loginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.fails[key] = append(l.recent(key, now), now)
}

func (l *loginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

// Sweep 清理过期记录，防止内存无限增长。
// Sweep drops stale entries so memory does not grow unbounded.
func (l *loginLimiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k := range l.fails {
		l.recent(k, now)
	}
}

func (s *Server) SweepLimiter() { s.limiter.Sweep() }
