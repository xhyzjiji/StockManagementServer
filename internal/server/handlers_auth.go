package server

import (
	"errors"
	"net/http"

	"private/construction_warehouse/internal/auth"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleAuthConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"allow_register":    s.cfg.Auth.AllowRegister,
		"token_ttl_seconds": int64(s.auth.TTL().Seconds()),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if s.limiter.Blocked(ip) {
		writeError(w, http.StatusTooManyRequests, "登录失败次数过多，请稍后再试")
		return
	}
	var c credentials
	if err := readJSON(w, r, &c); err != nil {
		fail(w, r, err)
		return
	}
	u, token, expires, err := s.auth.Login(r.Context(), c.Username, c.Password)
	if err != nil {
		if errors.Is(err, auth.ErrBadCredentials) {
			s.limiter.Fail(ip)
		}
		fail(w, r, err)
		return
	}
	s.limiter.Reset(ip)
	s.setTokenCookie(w, token, expires)
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "expires_at": expires.UnixMilli()})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Auth.AllowRegister {
		writeError(w, http.StatusForbidden, "未开放注册，请联系管理员")
		return
	}
	var c credentials
	if err := readJSON(w, r, &c); err != nil {
		fail(w, r, err)
		return
	}
	if _, err := s.auth.Register(r.Context(), c.Username, c.Password); err != nil {
		fail(w, r, err)
		return
	}
	u, token, expires, err := s.auth.Login(r.Context(), c.Username, c.Password)
	if err != nil {
		fail(w, r, err)
		return
	}
	s.setTokenCookie(w, token, expires)
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "expires_at": expires.UnixMilli()})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.cfg.Auth.CookieName); err == nil {
		if err := s.auth.Logout(r.Context(), c.Value); err != nil {
			fail(w, r, err)
			return
		}
	}
	s.clearCookie(w)
	writeOK(w)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": userOf(r)})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(w, r, &req); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.auth.ChangePassword(r.Context(), uid(r), tokenOf(r), req.OldPassword, req.NewPassword); err != nil {
		fail(w, r, err)
		return
	}
	writeOK(w)
}
