package main

import (
	"github.com/andy-zhangtao/logchain/logging"
	"fmt"
	"io/ioutil"
	"sync"
	"github.com/docker/docker/daemon/logger"
	"io"
	"path/filepath"
	"os"
	"github.com/pkg/errors"
	"github.com/docker/docker/daemon/logger/jsonfilelog"
)

type LogChain struct {
	mu     sync.Mutex
	logs   map[string]*logPair
	idx    map[string]*logPair
	logger logger.Logger
}

type logPair struct {
	jsonl   logger.Logger
	splunkl logger.Logger
	stream  io.ReadCloser
	info    logging.Info
}

func (lc *LogChain) Handler(lr logging.LogsRequest) error {
	lc.mu.Lock()
	if _, exists := lc.logs[lr.File]; exists {
		lc.mu.Unlock()
		return fmt.Errorf("logger for %q already exists", lr.File)
	}
	lc.mu.Unlock()

	if lr.Info.LogPath == "" {
		lr.Info.LogPath = filepath.Join("/var/log/docker", lr.Info.ContainerID)
	}

	if err := os.MkdirAll(filepath.Dir(lr.Info.LogPath), 0755); err != nil {
		return errors.Wrap(err, "error setting up logger dir")
	}

	jsonl, err := jsonfilelog.New()
	if err != nil {
		return errors.Wrap(err, "error creating jsonfile logger")
	}

	data, err := ioutil.ReadFile(lr.File)
	if err != nil {
		fmt.Printf("======handler==[%v]\n", err.Error())
		return err
	}
	fmt.Printf("======handler==[%s]\n", string(data))
	return nil
}

func (lc *LogChain) HandlerStop(logging.LogsRequest) error {
	fmt.Println("======handler stop")
	return nil
}
