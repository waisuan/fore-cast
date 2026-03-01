package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/handlers"
	"github.com/waisuan/alfred/internal/middlewares"
	"github.com/waisuan/alfred/internal/session"
)

// New builds the HTTP router with all routes and middlewares.
func New(d *deps.Dependencies, sessStore *session.Store, client *booker.Client) http.Handler {
	r := mux.NewRouter()
	r.Use(middlewares.CORS)

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}).Methods(http.MethodGet)

	authHandler := &handlers.AuthHandler{Booker: client, Store: sessStore}
	r.HandleFunc("/api/v1/auth/login", authHandler.Login).Methods(http.MethodPost)

	api := r.PathPrefix("/api/v1/").Subrouter()
	api.Use(middlewares.SessionAuth(sessStore))

	api.HandleFunc("/auth/logout", authHandler.Logout).Methods(http.MethodPost)
	api.HandleFunc("/auth/me", authHandler.Me).Methods(http.MethodGet)

	slotsHandler := &handlers.SlotsHandler{Booker: client}
	api.HandleFunc("/slots", slotsHandler.Slots).Methods(http.MethodGet)

	bookingHandler := &handlers.BookingHandler{Booker: client}
	api.HandleFunc("/booking", bookingHandler.GetBooking).Methods(http.MethodGet)
	api.HandleFunc("/booking/check-status", bookingHandler.CheckStatus).Methods(http.MethodPost)
	api.HandleFunc("/booking/book", bookingHandler.Book).Methods(http.MethodPost)

	historyHandler := &handlers.HistoryHandler{Service: d.Service}
	api.HandleFunc("/history", historyHandler.GetHistory).Methods(http.MethodGet)

	presetHandler := &handlers.PresetHandler{Service: d.Service, EncryptionKey: d.Config.EncryptionKey}
	api.HandleFunc("/preset", presetHandler.GetPreset).Methods(http.MethodGet)
	api.HandleFunc("/preset", presetHandler.SavePreset).Methods(http.MethodPut)

	return r
}
