package logging

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/docker/go-plugins-helpers/sdk"
	"time"
	"fmt"
)

const (
	manifest     = `{"Implements": ["LoggingDriver"]}`
	startLogging = "/LogDriver.StartLogging"
	stopLogging  = "/LogDriver.StopLogging"
)

type Info struct {
	Config              map[string]string
	ContainerID         string
	ContainerName       string
	ContainerEntrypoint string
	ContainerArgs       []string
	ContainerImageID    string
	ContainerImageName  string
	ContainerCreated    time.Time
	ContainerEnv        []string
	ContainerLabels     map[string]string
	LogPath             string
	DaemonName          string
}

// LogsRequest is the plugin secret request
type LogsRequest struct {
	File string
	Info Info
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
func NewHandler(plugin *Plugin) *Handler {
	h := &Handler{plugin, sdk.NewHandler(manifest)}
	h.initMux()
	return h
}

func (h *Handler) initMux() {
	h.HandleFunc(startLogging, func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("start")
		var req LogsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Info.ContainerID == "" {
			respond(errors.New("must provide container id in log context"), w)
			return
		}
		fmt.Printf("in startLogging [%v]\n", req.Info)
		fmt.Printf("in startLogging [%v]\n", req.File)

		err := (*h.plugin).Handler(req)
		respond(err, w)
	})
	h.HandleFunc(stopLogging, func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("stop")
		var req LogsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

	fmt.Printf("Send Response [%s]\n", string(data))
	_, err = w.Write(data)
	if err != nil {
		fmt.Printf("Send Response error[%s]\n", err.Error())
	}
}
