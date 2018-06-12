package main

import (
	"fmt"
	"os"
	"github.com/andy-zhangtao/logchain/logging"
	"strconv"
	"os/user"
	"github.com/Sirupsen/logrus"
	"github.com/andy-zhangtao/logchain/log"
)

const socketAddress = "/run/docker/plugins/logchain.sock"

var logLevels = map[string]logrus.Level{
	"debug": logrus.DebugLevel,
	"info":  logrus.InfoLevel,
	"warn":  logrus.WarnLevel,
	"error": logrus.ErrorLevel,
}

func main() {
	logrus.Println("==LogChain 1.2.0==")
	levelVal := os.Getenv("LOG_LEVEL")
	if levelVal == "" {
		levelVal = "info"
	}

	logrus.WithFields(log.Z.Fields(logrus.Fields{"LogChain Start Work": true})).Info(log.ModuleName)

	if level, exists := logLevels[levelVal]; exists {
		logrus.SetLevel(level)
	} else {
		fmt.Fprintln(os.Stderr, "invalid log level: ", levelVal)
		os.Exit(1)
	}
	u, _ := user.Lookup("root")
	gid, _ := strconv.Atoi(u.Gid)

	lc := LogChain{
		logs: make(map[string]*logPair),
		idx:  make(map[string]*logPair),
		stop: make(chan int),
	}

	h := logging.NewHandler(&lc)

	if err := h.ServeUnix(socketAddress, gid); err != nil {
		panic(err)
	}

	logrus.WithFields(log.Z.Fields(logrus.Fields{"LogChain End Work": "ByeBye"})).Info(log.ModuleName)
}
