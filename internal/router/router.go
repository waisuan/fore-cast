package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/handlers"
	"github.com/waisuan/alfred/internal/middlewares"
)

// New builds the HTTP router with all routes and middlewares.
func New(d *deps.Dependencies) http.Handler {
	r := mux.NewRouter()
	r.Use(middlewares.Logging())
	r.Use(middlewares.CORS)
	r.Use(middlewares.BodyLimit)
	r.Use(middlewares.ErrorMask())

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}).Methods(http.MethodGet)

	authHandler := &handlers.AuthHandler{Credentials: d.Credentials, Store: d.Store, EncryptionKey: d.Config.EncryptionKey}
	r.HandleFunc("/api/v1/auth/login", authHandler.Login).Methods(http.MethodPost)

	api := r.PathPrefix("/api/v1/").Subrouter()
	api.Use(middlewares.SessionAuth(d.Store, d.Credentials))

	adminHandler := &handlers.AdminHandler{
		Config:      d.Config,
		Booker:      d.Booker,
		Credentials: d.Credentials,
		Preset:      d.Preset,
		PG:          d.PG,
	}
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(middlewares.RequireAdmin)
	admin.HandleFunc("/register", adminHandler.Register).Methods(http.MethodPost)
	admin.HandleFunc("/users", adminHandler.ListUsers).Methods(http.MethodGet)
	admin.HandleFunc("/users/{username}/role", adminHandler.SetUserRole).Methods(http.MethodPut)
	admin.HandleFunc("/users/{username}", adminHandler.DeleteUser).Methods(http.MethodDelete)
	admin.HandleFunc("/presets/{username}", adminHandler.DeletePreset).Methods(http.MethodDelete)

	api.HandleFunc("/auth/logout", authHandler.Logout).Methods(http.MethodPost)
	api.HandleFunc("/auth/me", authHandler.Me).Methods(http.MethodGet)

	// Member-only: booking, history, presets (admins use /admin only in the UI; API is blocked too)
	member := api.PathPrefix("").Subrouter()
	member.Use(middlewares.DenyAdmin)

	historyHandler := &handlers.HistoryHandler{Service: d.History}
	member.HandleFunc("/history", historyHandler.GetHistory).Methods(http.MethodGet)

	presetHandler := &handlers.PresetHandler{Service: d.Preset}
	member.HandleFunc("/preset", presetHandler.GetPreset).Methods(http.MethodGet)
	member.HandleFunc("/preset", presetHandler.SavePreset).Methods(http.MethodPut)
	member.HandleFunc("/preset/cancel", presetHandler.CancelRun).Methods(http.MethodPost)

	bookerAPI := member.PathPrefix("").Subrouter()
	bookerAPI.Use(middlewares.TokenRefresh(d.Booker, d.Credentials, d.Config.EncryptionKey))
	slotsHandler := &handlers.SlotsHandler{Booker: d.Booker}
	bookerAPI.HandleFunc("/slots", slotsHandler.Slots).Methods(http.MethodGet)
	bookingHandler := &handlers.BookingHandler{Booker: d.Booker}
	bookerAPI.HandleFunc("/booking", bookingHandler.GetBooking).Methods(http.MethodGet)
	bookerAPI.HandleFunc("/booking/check-status", bookingHandler.CheckStatus).Methods(http.MethodPost)
	bookerAPI.HandleFunc("/booking/book", bookingHandler.Book).Methods(http.MethodPost)

	return r
}
