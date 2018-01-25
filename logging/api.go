package logging

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/go-plugins-helpers/sdk"
)

const (
	manifest     = `{"Implements": ["LogDriver"]}`
	startLogging = "/LogDriver.StartLogging"
	stopLogging  = "/LogDriver.StopLogging"
)

// LogsRequest is the plugin secret request
type LogsRequest struct {
	File string
	Info logger.Info
}

// Response contains the plugin secret value
type Response struct {
	Err string `json:",omitempty"` // Err is the error response of the plugin
}

// Plugin represent the interface a plugin must fulfill.
type Plugin interface {
	Handler(LogsRequest) error
	HandlerStop(LogsRequest) error
}

// Handler forwards requests and responses between the docker daemon and the plugin.
type Handler struct {
	plugin Plugin
	sdk.Handler
}

// NewHandler initializes the request handler with a driver implementation.
func NewHandler(plugin Plugin) *Handler {
	h := &Handler{plugin, sdk.NewHandler(manifest)}
	h.initMux()
	return h
}

func (h *Handler) initMux() {
	h.HandleFunc(startLogging, func(w http.ResponseWriter, r *http.Request) {
		var req LogsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Info.ContainerID == "" {
			respond(errors.New("must provide container id in log context"), w)
			return
		}

		err := h.plugin.Handler(req)
		respond(err, w)
	})
	h.HandleFunc(stopLogging, func(w http.ResponseWriter, r *http.Request) {
		var req LogsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err := h.plugin.HandlerStop(req)
		respond(err, w)
	})
}

func respond(err error, w http.ResponseWriter) {
	var res Response
	if err != nil {
		res.Err = err.Error()
	}
	json.NewEncoder(w).Encode(&res)
}
