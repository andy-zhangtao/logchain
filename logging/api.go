package logging

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/docker/go-plugins-helpers/sdk"
	"fmt"
	"github.com/docker/docker/daemon/logger"
	"strings"
)

const (
	manifest     = `{"Implements": ["LoggingDriver"]}`
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
	Err string `json:"err"` // Err is the error response of the plugin
}

// Plugin represent the interface a plugin must fulfill.
type Plugin interface {
	Handler(LogsRequest) error
	HandlerStop(LogsRequest) error
}

// Handler forwards requests and responses between the docker daemon and the plugin.
type Handler struct {
	plugin *Plugin
	sdk.Handler
}

// NewHandler initializes the request handler with a driver implementation.
func NewHandler(plugin Plugin) *Handler {
	h := &Handler{&plugin, sdk.NewHandler(manifest)}
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

		parseParaViaEnv(&req)
		err := (*h.plugin).Handler(req)
		respond(err, w)
	})
	h.HandleFunc(stopLogging, func(w http.ResponseWriter, r *http.Request) {
		var req LogsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		parseParaViaEnv(&req)
		err := (*h.plugin).HandlerStop(req)
		respond(err, w)
	})

}

func respond(err error, w http.ResponseWriter) {

	var data []byte
	if err != nil {
		res := Response{
			Err: err.Error(),
		}
		data, err = json.Marshal(&res)
		if err != nil {
			data = []byte(err.Error())
		}
	} else {
		data = []byte("{}")
	}

	_, err = w.Write(data)
	if err != nil {
		fmt.Printf("Send Response error[%s]\n", err.Error())
	}
}

// parsePara
// If we manager docker via systemd. There has no way to configure parameter in systemd. So we will meet CAE issue (Chicken and eggs.)
// Then we parse parameter via env.
func parseParaViaEnv(lr *LogsRequest) {
	log_opt := ""
	for _, s := range lr.Info.ContainerEnv {
		if strings.Contains(s, "log_opt=") {
			log_opt = s[len("log_opt="):]
			break
		}
	}

	if log_opt == "" {
		return
	}

	for _, lg := range strings.Split(log_opt, ";") {
		lgs := strings.Split(lg, "--log-opt")

		if len(lgs) != 2 {
			continue
		}

		lgk := strings.Split(lgs[1], "=")
		if len(lgk) != 2 {
			continue
		}

		lr.Info.Config[strings.TrimSpace(lgk[0])] = strings.TrimSpace(lgk[1])
	}
}
