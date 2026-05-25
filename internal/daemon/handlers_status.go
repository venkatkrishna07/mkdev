package daemon

import "net/http"

func (d *Daemon) handleStatus(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, d.Status())
}

func (d *Daemon) handleShutdown(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusAccepted)
	go d.invokeShutdownHook()
}
