package daemon

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("daemon: WriteJSON encode failed", "err", err)
	}
}

func WriteError(w http.ResponseWriter, err error) {
	var apiErr api.Error
	if errors.As(err, &apiErr) {
		WriteJSON(w, api.HTTPStatus(apiErr.Code), apiErr)
		return
	}
	WriteJSON(w, http.StatusInternalServerError, api.Error{
		Code:    "internal",
		Message: err.Error(),
	})
}

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("daemon: panic in handler", "path", r.URL.Path, "panic", rec)
				WriteError(w, errors.New("internal panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
