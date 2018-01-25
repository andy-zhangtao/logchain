package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/andy-zhangtao/logchain/logging"
)

type LogChain struct {
}

func (lc LogChain) Handler(logging.LogsRequest) error {
	logrus.Debug("======handler")
	return nil
}

func (lc LogChain) HandlerStop(logging.LogsRequest) error {
	logrus.Debug("======handler stop")
	return nil
}
