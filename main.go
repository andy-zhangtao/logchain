package main

import (
	"fmt"
	"os"
	"github.com/Sirupsen/logrus"
	"github.com/andy-zhangtao/logchain/logging"
	"strconv"
	"os/user"
)

const socketAddress = "/run/docker/plugins/logchain.sock"

var logLevels = map[string]logrus.Level{
	"debug": logrus.DebugLevel,
	"info":  logrus.InfoLevel,
	"warn":  logrus.WarnLevel,
	"error": logrus.ErrorLevel,
}

func main() {
	levelVal := os.Getenv("LOG_LEVEL")
	if levelVal == "" {
		levelVal = "info"
	}
	if level, exists := logLevels[levelVal]; exists {
		logrus.SetLevel(level)
	} else {
		fmt.Fprintln(os.Stderr, "invalid log level: ", levelVal)
		os.Exit(1)
	}
	u, _ := user.Lookup("root")
	gid, _ := strconv.Atoi(u.Gid)

	h := logging.NewHandler(LogChain{})

	if err := h.ServeUnix(socketAddress, gid); err != nil {
		panic(err)
	}

	fmt.Println("===========end==============")
}
