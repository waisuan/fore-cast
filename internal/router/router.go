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

	adminHandler := &handlers.AdminHandler{Config: d.Config, Booker: d.Booker, Credentials: d.Credentials}
	r.HandleFunc("/api/v1/admin/register", adminHandler.Register).Methods(http.MethodPost)

	api := r.PathPrefix("/api/v1/").Subrouter()
	api.Use(middlewares.SessionAuth(d.Store))

	api.HandleFunc("/auth/logout", authHandler.Logout).Methods(http.MethodPost)
	api.HandleFunc("/auth/me", authHandler.Me).Methods(http.MethodGet)

	// Routes that need 3rd party token (TokenRefresh obtains it on-demand from credentials)
	bookerAPI := api.PathPrefix("").Subrouter()
	bookerAPI.Use(middlewares.TokenRefresh(d.Booker, d.Credentials, d.Config.EncryptionKey))
	slotsHandler := &handlers.SlotsHandler{Booker: d.Booker}
	bookerAPI.HandleFunc("/slots", slotsHandler.Slots).Methods(http.MethodGet)
	bookingHandler := &handlers.BookingHandler{Booker: d.Booker}
	bookerAPI.HandleFunc("/booking", bookingHandler.GetBooking).Methods(http.MethodGet)
	bookerAPI.HandleFunc("/booking/check-status", bookingHandler.CheckStatus).Methods(http.MethodPost)
	bookerAPI.HandleFunc("/booking/book", bookingHandler.Book).Methods(http.MethodPost)

	historyHandler := &handlers.HistoryHandler{Service: d.History}
	api.HandleFunc("/history", historyHandler.GetHistory).Methods(http.MethodGet)

	presetHandler := &handlers.PresetHandler{Service: d.Preset}
	api.HandleFunc("/preset", presetHandler.GetPreset).Methods(http.MethodGet)
	api.HandleFunc("/preset", presetHandler.SavePreset).Methods(http.MethodPut)

	return r
}
