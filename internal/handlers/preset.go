package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/notify"
	"github.com/waisuan/alfred/internal/preset"
)

// PresetHandler handles /api/v1/preset endpoints.
type PresetHandler struct {
	Service             preset.Service
	MaxParallelSlotsMax int
}

// PresetDefaults contains the server-side default values for preset fields.
type PresetDefaults struct {
	Course           string `json:"course"`
	Cutoff           string `json:"cutoff"`
	RetryInterval    string `json:"retry_interval"`
	MinRetryInterval string `json:"min_retry_interval"`
	Timeout          string `json:"timeout"`
	MaxParallelSlots int    `json:"max_parallel_slots"`
}

// PresetResponse is the JSON response for GET /api/v1/preset.
type PresetResponse struct {
	UserName            string         `json:"user_name"`
	Course              string         `json:"course"`
	Cutoff              string         `json:"cutoff"`
	RetryInterval       string         `json:"retry_interval"`
	Timeout             string         `json:"timeout"`
	MaxParallelSlots    int            `json:"max_parallel_slots"`
	NtfyTopic           string         `json:"ntfy_topic"`
	EnableNotifications bool           `json:"enable_notifications"`
	Enabled             bool           `json:"enabled"`
	Defaults            PresetDefaults `json:"defaults"`
	LastRunStatus       string         `json:"last_run_status"`
	LastRunMessage      string         `json:"last_run_message"`
	LastRunAt           *string        `json:"last_run_at"`
}

// PresetRequest is the JSON body for PUT /api/v1/preset.
type PresetRequest struct {
	Course              string `json:"course"`
	Cutoff              string `json:"cutoff"`
	RetryInterval       string `json:"retry_interval"`
	Timeout             string `json:"timeout"`
	MaxParallelSlots    *int   `json:"max_parallel_slots"`
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
		Course:           "Auto (by day of week)",
		Cutoff:           preset.DefaultCutoff,
		RetryInterval:    preset.DefaultRetryInterval,
		MinRetryInterval: preset.MinRetryInterval,
		Timeout:          preset.DefaultTimeout,
		MaxParallelSlots: preset.DefaultMaxParallelSlots,
	}

	existing, err := h.Service.GetPreset(u.UserName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := PresetResponse{Defaults: defaults, LastRunStatus: string(preset.RunStatusIdle)}
	if existing == nil {
		resp.UserName = u.UserName
		resp.Cutoff = preset.DefaultCutoff
		resp.RetryInterval = preset.DefaultRetryInterval
		resp.Timeout = preset.DefaultTimeout
		resp.MaxParallelSlots = preset.DefaultMaxParallelSlots
	} else {
		resp.UserName = existing.UserName
		resp.Course = existing.Course.String
		resp.Cutoff = existing.Cutoff
		resp.RetryInterval = existing.RetryInterval
		resp.Timeout = existing.Timeout
		resp.MaxParallelSlots = existing.MaxParallelSlots
		resp.NtfyTopic = existing.NtfyTopic.String
		resp.EnableNotifications = existing.NtfyTopic.Valid && existing.NtfyTopic.String != ""
		resp.Enabled = existing.Enabled
		resp.LastRunStatus = existing.LastRunStatus
		resp.LastRunMessage = existing.LastRunMessage
		if existing.LastRunAt.Valid {
			t := existing.LastRunAt.Time.Format("2006-01-02T15:04:05Z07:00")
			resp.LastRunAt = &t
		}
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

	ntfyTopic := resolveNtfyTopic(existing, u.UserName, req.EnableNotifications)

	var maxParallel int
	if req.MaxParallelSlots != nil {
		v := *req.MaxParallelSlots
		if v < 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "max_parallel_slots must be at least 1"})
			return
		}
		if h.MaxParallelSlotsMax >= 1 && v > h.MaxParallelSlotsMax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "max_parallel_slots must be at most " + strconv.Itoa(h.MaxParallelSlotsMax)})
			return
		}
		maxParallel = v
	} else if existing != nil {
		maxParallel = existing.MaxParallelSlots
	} else {
		maxParallel = preset.DefaultMaxParallelSlots
	}

	p := preset.Preset{
		UserName:         u.UserName,
		Course:           sql.NullString{String: req.Course, Valid: req.Course != ""},
		Cutoff:           req.Cutoff,
		RetryInterval:    req.RetryInterval,
		Timeout:          req.Timeout,
		MaxParallelSlots: maxParallel,
		NtfyTopic:        ntfyTopic,
		Enabled:          req.Enabled,
	}
	if p.Cutoff == "" {
		p.Cutoff = preset.DefaultCutoff
	}
	if p.RetryInterval == "" {
		p.RetryInterval = preset.DefaultRetryInterval
	} else {
		d, err := time.ParseDuration(p.RetryInterval)
		if err != nil {
			p.RetryInterval = preset.DefaultRetryInterval
		} else if d < preset.MinRetryIntervalDuration {
			p.RetryInterval = preset.MinRetryInterval
		}
	}
	if p.Timeout == "" {
		p.Timeout = preset.DefaultTimeout
	}

	if err := h.Service.UpsertPreset(p); err != nil {
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
func resolveNtfyTopic(existing *preset.Preset, userName string, enable *bool) sql.NullString {
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
