package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/preset"
	"github.com/waisuan/alfred/internal/user"
)

// AdminHandler handles admin-only endpoints (RequireAdmin middleware).
type AdminHandler struct {
	Config      *deps.Config
	Booker      booker.ClientInterface
	Credentials credentials.Service
	Preset      preset.Service
	PG          *sql.DB
}

// RegisterRequest is the body for POST /api/v1/admin/register.
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register creates or updates credentials for a user (new users are NON_ADMIN).
func (h *AdminHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

	if err := h.Credentials.Upsert(req.Username, passwordEnc, context.RoleNonAdmin); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "registered", "username": req.Username})
}

// DeleteUser removes credentials, preset, sessions, and booking history for a user.
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.PG == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}
	userName := mux.Vars(r)["username"]
	if userName == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}
	if err := user.DeleteUser(h.PG, userName); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "username": userName})
}

// SetRoleRequest is the body for PUT /api/v1/admin/users/{username}/role.
type SetRoleRequest struct {
	Role string `json:"role"`
}

// ListUsers returns all users (user_name, role, created_at). No passwords.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.PG == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}
	users, err := user.List(h.PG)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"users": users})
}

// SetUserRole sets ADMIN or NON_ADMIN for an existing user.
func (h *AdminHandler) SetUserRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.PG == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}
	userName := mux.Vars(r)["username"]
	if userName == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}
	var req SetRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		http.Error(w, "role required", http.StatusBadRequest)
		return
	}
	if err := user.SetRole(h.PG, userName, req.Role); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, user.ErrInvalidRole) {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated", "username": userName, "role": req.Role})
}

// DeletePreset removes the booking preset row for a user; credentials remain so they can log in again.
func (h *AdminHandler) DeletePreset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Preset == nil {
		http.Error(w, "preset service not configured", http.StatusInternalServerError)
		return
	}
	userName := mux.Vars(r)["username"]
	if userName == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}
	if err := h.Preset.DeleteByUserName(userName); err != nil {
		if errors.Is(err, preset.ErrPresetNotFound) {
			http.Error(w, "preset not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "username": userName})
}
