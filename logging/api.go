package logging

import (
	"encoding/json"
	"errors"
	"net/http"
	"github.com/docker/go-plugins-helpers/sdk"

	"github.com/docker/docker/daemon/logger"
	"strings"
	"reflect"
	"encoding/binary"
	protoio "github.com/gogo/protobuf/io"

	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/andy-zhangtao/gogather/time"
	"strconv"
	"fmt"
	"github.com/Sirupsen/logrus"
)

const (
	manifest     = `{"Implements": ["LoggingDriver"]}`
	startLogging = "/LogDriver.StartLogging"
	stopLogging  = "/LogDriver.StopLogging"
	readLog      = "/LogDriver.ReadLogs"
)

// LogsRequest is the plugin secret request
type LogsRequest struct {
	File string
	Info logger.Info
}

type LogsReadRequest struct {
	Config logger.ReadConfig
	Info   logger.Info
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
	// enable HandlerRead
	readHeader := reflect.ValueOf(plugin).MethodByName("HandlerRead")
	if readHeader.IsValid() {

		h.HandleFunc("/LogDriver.Capabilities", func(w http.ResponseWriter, r *http.Request) {

			type logPluginProxyCapabilitiesResponse struct {
				Cap logger.Capability
				Err string
			}

			re := logPluginProxyCapabilitiesResponse{Cap: logger.Capability{ReadLogs: true}, Err: ""}
			data, _ := json.Marshal(&re)

			w.Write(data)
		})

		h.HandleFunc(readLog, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-json-stream")
			var config LogsReadRequest
			if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			result := readHeader.Call([]reflect.Value{reflect.ValueOf(config)})
			err := result[1].Interface()
			if err != nil {
				http.Error(w, err.(error).Error(), http.StatusBadRequest)
				return
			}

			watcher := result[0].Interface().(*logger.LogWatcher)
			writer := protoio.NewUint32DelimitedWriter(w, binary.BigEndian)
			for {
				select {
				case m := <-watcher.Msg:
					nn, err := strconv.ParseInt(time.GetTimeStamp(19), 10, 64)
					if err != nil {
						fmt.Errorf("Get TimeNano Error[%s]\n", err.Error())
					}
					msg := logdriver.LogEntry{
						Source:   "stdout",
						Partial:  false,
						TimeNano: nn,
					}
					if m == nil {
						msg.Partial = true
						msg.Line = []byte("\n")
						writer.WriteMsg(&msg)
						writer.Close()
						return
					}
					msg.Line = m.Line
					err = writer.WriteMsg(&msg)
					if err != nil {
						fmt.Errorf("Write Msg Error[%s]\n", err.Error())
					}

				case <-watcher.Err:
					return
				}
			}
		})
	}

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
		fmt.Println("Not Find log_opt. Then use default logger json-file")
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

	driver := "graylog"
	for _, s := range lr.Info.ContainerEnv {
		if strings.Contains(s, "LOGCHAIN_DRIVER") {
			driver = s[len("LOGCHAIN_DRIVER="):]
			break
		}
	}

	lr.Info.Config["driver"] = driver
	logrus.WithFields(logrus.Fields{"Info": lr.Info}).Info("logchain")
}
