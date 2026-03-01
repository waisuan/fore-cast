package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/db"
	"github.com/waisuan/alfred/internal/notify"
)

const (
	DefaultCutoff        = "8:15"
	DefaultRetryInterval = 1
	DefaultTimeout       = "10m"
)

// PresetHandler handles /api/v1/preset endpoints.
type PresetHandler struct {
	Service       db.ServiceInterface
	EncryptionKey string
}

// PresetDefaults contains the server-side default values for preset fields.
type PresetDefaults struct {
	Course        string `json:"course"`
	Cutoff        string `json:"cutoff"`
	RetryInterval int    `json:"retry_interval"`
	Timeout       string `json:"timeout"`
}

// PresetResponse is the JSON response for GET /api/v1/preset.
type PresetResponse struct {
	UserName            string         `json:"user_name"`
	Course              string         `json:"course"`
	Cutoff              string         `json:"cutoff"`
	RetryInterval       int            `json:"retry_interval"`
	Timeout             string         `json:"timeout"`
	NtfyTopic           string         `json:"ntfy_topic"`
	EnableNotifications bool           `json:"enable_notifications"`
	Enabled             bool           `json:"enabled"`
	Defaults            PresetDefaults `json:"defaults"`
}

// PresetRequest is the JSON body for PUT /api/v1/preset.
type PresetRequest struct {
	Course              string `json:"course"`
	Cutoff              string `json:"cutoff"`
	RetryInterval       int    `json:"retry_interval"`
	Timeout             string `json:"timeout"`
	EnableNotifications *bool  `json:"enable_notifications"`
	Enabled             bool   `json:"enabled"`
}

// GetPreset handles GET /api/v1/preset.
func (h *PresetHandler) GetPreset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defaults := PresetDefaults{
		Course:        "Auto (by day of week)",
		Cutoff:        DefaultCutoff,
		RetryInterval: DefaultRetryInterval,
		Timeout:       DefaultTimeout,
	}

	preset, err := h.Service.GetPreset(u.UserName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := PresetResponse{Defaults: defaults}
	if preset == nil {
		resp.UserName = u.UserName
		resp.Cutoff = DefaultCutoff
		resp.RetryInterval = DefaultRetryInterval
		resp.Timeout = DefaultTimeout
	} else {
		resp.UserName = preset.UserName
		resp.Course = preset.Course.String
		resp.Cutoff = preset.Cutoff
		resp.RetryInterval = preset.RetryInterval
		resp.Timeout = preset.Timeout
		resp.NtfyTopic = preset.NtfyTopic.String
		resp.EnableNotifications = preset.NtfyTopic.Valid && preset.NtfyTopic.String != ""
		resp.Enabled = preset.Enabled
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SavePreset handles PUT /api/v1/preset.
func (h *PresetHandler) SavePreset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req PresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	existing, err := h.Service.GetPreset(u.UserName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var passwordEnc string
	if existing != nil && existing.PasswordEnc != "" {
		passwordEnc = existing.PasswordEnc
	} else {
		if u.Password == "" {
			http.Error(w, "session missing password — please log in again", http.StatusUnauthorized)
			return
		}
		if h.EncryptionKey == "" {
			http.Error(w, "encryption key not configured", http.StatusInternalServerError)
			return
		}
		enc, err := crypto.Encrypt(u.Password, h.EncryptionKey)
		if err != nil {
			http.Error(w, "failed to encrypt password", http.StatusInternalServerError)
			return
		}
		passwordEnc = enc
	}

	ntfyTopic := resolveNtfyTopic(existing, u.UserName, req.EnableNotifications)

	preset := db.Preset{
		UserName:      u.UserName,
		PasswordEnc:   passwordEnc,
		Course:        sql.NullString{String: req.Course, Valid: req.Course != ""},
		Cutoff:        req.Cutoff,
		RetryInterval: req.RetryInterval,
		Timeout:       req.Timeout,
		NtfyTopic:     ntfyTopic,
		Enabled:       req.Enabled,
	}
	if preset.Cutoff == "" {
		preset.Cutoff = DefaultCutoff
	}
	if preset.RetryInterval < 1 {
		preset.RetryInterval = DefaultRetryInterval
	}
	if preset.Timeout == "" {
		preset.Timeout = DefaultTimeout
	}

	if err := h.Service.UpsertPreset(preset); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// resolveNtfyTopic determines the ntfy topic based on the user's notification
// preference and their existing preset. If enabled and no topic exists yet,
// a new one is generated. If disabled, the topic is cleared. If the flag is
// nil (not sent), the existing topic is preserved.
func resolveNtfyTopic(existing *db.Preset, userName string, enable *bool) sql.NullString {
	if enable == nil {
		if existing != nil {
			return existing.NtfyTopic
		}
		return sql.NullString{}
	}

	if !*enable {
		return sql.NullString{}
	}

	if existing != nil && existing.NtfyTopic.Valid && existing.NtfyTopic.String != "" {
		return existing.NtfyTopic
	}

	topic := notify.GenerateTopic(userName)
	return sql.NullString{String: topic, Valid: true}
}
