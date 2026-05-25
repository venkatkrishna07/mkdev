package daemon

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

var errExtraJSON = errors.New("trailing JSON after value")

// NewHandler returns the daemon's http.Handler with all routes registered.
func NewHandler(d *Daemon) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/routes", d.handleListRoutes)
	mux.HandleFunc("POST /v1/routes", d.handleAddRoute)
	mux.HandleFunc("GET /v1/routes/{name}", d.handleGetRoute)
	mux.HandleFunc("PATCH /v1/routes/{name}", d.handlePatchRoute)
	mux.HandleFunc("DELETE /v1/routes/{name}", d.handleDeleteRoute)
	mux.HandleFunc("POST /v1/routes/{name}/share", d.handleShareRoute)
	mux.HandleFunc("GET /v1/events", d.handleEvents)
	mux.HandleFunc("GET /v1/status", d.handleStatus)
	mux.HandleFunc("POST /v1/shutdown", d.handleShutdown)
	return Recoverer(mux)
}

func (d *Daemon) handleListRoutes(w http.ResponseWriter, _ *http.Request) {
	routes, err := d.Routes()
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, routes)
}

func (d *Daemon) handleAddRoute(w http.ResponseWriter, r *http.Request) {
	var in api.Route
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, api.Error{Code: api.CodeRouteInvalidName, Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := d.AddRoute(in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, out)
}

func (d *Daemon) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	all, err := d.Routes()
	if err != nil {
		WriteError(w, err)
		return
	}
	for _, rt := range all {
		if rt.Name == name {
			WriteJSON(w, http.StatusOK, rt)
			return
		}
	}
	WriteError(w, api.Error{Code: api.CodeRouteNotFound, Message: "no route " + name})
}

func (d *Daemon) handlePatchRoute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var raw struct {
		Target *string    `json:"target"`
		Share  *api.Share `json:"share"`
	}
	if err := decodeJSON(r, &raw); err != nil {
		WriteError(w, api.Error{Code: api.CodeRouteInvalidTarget, Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := d.EditRoute(name, RouteEdit{Target: raw.Target, Share: raw.Share})
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

func (d *Daemon) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := d.RemoveRoute(name); err != nil {
		WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleShareRoute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var raw struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &raw); err != nil {
		WriteError(w, api.Error{Code: api.CodeRouteInvalidTarget, Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := d.ToggleShare(name, raw.Enabled)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

func decodeJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errExtraJSON
	}
	return nil
}
