package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/deps"
)

// AdminHandler handles admin-only endpoints.
type AdminHandler struct {
	Config      *deps.Config
	Booker      booker.ClientInterface
	Credentials credentials.Service
}

// RegisterRequest is the body for POST /api/v1/admin/register.
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register creates or updates a preset for a user. Requires HTTP Basic Auth with ADMIN_USER/ADMIN_PASSWORD.
func (h *AdminHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.validateBasicAuth(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Admin"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.Config.EncryptionKey == "" {
		http.Error(w, "encryption key not configured", http.StatusInternalServerError)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}

	_, err := h.Booker.Login(req.Username, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	passwordEnc, err := crypto.Encrypt(req.Password, h.Config.EncryptionKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.Credentials.Upsert(req.Username, passwordEnc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "registered", "username": req.Username})
}

func (h *AdminHandler) validateBasicAuth(r *http.Request) bool {
	if h.Config.AdminUser == "" || h.Config.AdminPassword == "" {
		return false
	}
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Basic ") {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		return false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return false
	}
	return parts[0] == h.Config.AdminUser && parts[1] == h.Config.AdminPassword
}
