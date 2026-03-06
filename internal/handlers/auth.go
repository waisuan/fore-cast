package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/middlewares"
	"github.com/waisuan/alfred/internal/session"
)

// AuthHandler handles login, logout, and me.
type AuthHandler struct {
	Booker booker.ClientInterface
	Store  *session.Store
}

// LoginRequest is the body for POST /api/v1/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the response for login success.
type LoginResponse struct {
	User struct {
		Username string `json:"username"`
	} `json:"user"`
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}
	token, err := h.Booker.Login(req.Username, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	sid, err := h.Store.Create(token, req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middlewares.SessionCookieName(),
		Value:    sid,
		Path:     "/",
		MaxAge:   int(h.Store.TTL().Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoginResponse{
		User: struct {
			Username string `json:"username"`
		}{Username: req.Username},
	})
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(middlewares.SessionCookieName())
	if cookie != nil && cookie.Value != "" {
		h.Store.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middlewares.SessionCookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	w.WriteHeader(http.StatusOK)
}

// Me handles GET /api/v1/auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoginResponse{
		User: struct {
			Username string `json:"username"`
		}{Username: u.UserName},
	})
}
