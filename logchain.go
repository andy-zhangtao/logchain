package main

import (
	"github.com/andy-zhangtao/logchain/logging"
	"fmt"
	"sync"
	"github.com/docker/docker/daemon/logger"
	"io"
	"path/filepath"
	"os"
	"github.com/pkg/errors"
	"github.com/docker/docker/daemon/logger/jsonfilelog"
	"github.com/Sirupsen/logrus"
	"syscall"
	"github.com/tonistiigi/fifo"
	"context"
	"encoding/binary"
	protoio "github.com/gogo/protobuf/io"
	"github.com/docker/docker/api/types/plugins/logdriver"
	"time"
	"strings"
)

type LogChain struct {
	mu     sync.Mutex
	logs   map[string]*logPair
	idx    map[string]*logPair
	logger logger.Logger
}

type logPair struct {
	jsonl  logger.Logger
	driver logger.Logger
	stream io.ReadCloser
	info   logger.Info
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

	jsonl, err := jsonfilelog.New(lr.Info)
	if err != nil {
		return errors.Wrap(err, "error creating jsonfile logger")
	}

	log, err := New(lr.Info)
	if err != nil {
		return errors.Wrap(err, "error creating logger driver")
	}

	logrus.WithField("id", lr.Info.ContainerID).WithField("file", lr.File).WithField("logpath", lr.Info.LogPath).Debugf("Start logging")

	f, err := fifo.OpenFifo(context.Background(), lr.File, syscall.O_RDONLY, 0700)
	if err != nil {
		return errors.Wrapf(err, "error opening logger fifo: %q", lr.File)
	}

	lc.mu.Lock()

	lf := &logPair{jsonl, log, f, lr.Info}

	lc.logs[lr.File] = lf
	lc.idx[lr.Info.ContainerID] = lf
	lc.mu.Unlock()

	go consumeLog(lf)
	fmt.Println("log startLogging end")
	return nil
}

func (lc *LogChain) HandlerStop(logging.LogsRequest) error {
	fmt.Println("======handler stop")
	return nil
}

func consumeLog(lf *logPair) {
	dec := protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
	defer dec.Close()
	var buf logdriver.LogEntry
	for {
		if err := dec.ReadMsg(&buf); err != nil {
			if err == io.EOF {
				logrus.WithField("id", lf.info.ContainerID).WithError(err).Debug("shutting down log logger")
				lf.stream.Close()
				return
			}
			dec = protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
		}

		//fmt.Printf("Receive [%s] \n", buf.String())
		if sendMessage(lf.driver, &buf, lf.info.ContainerID) == false {
			continue
		}

		//if sendMessage(lf.jsonl, &buf, lf.info.ContainerID) == false {
		//	continue
		//}

		buf.Reset()
	}
}

func sendMessage(l logger.Logger, buf *logdriver.LogEntry, containerid string) bool {
	var msg logger.Message
	msg.Line = buf.Line
	msg.Source = buf.Source
	msg.Partial = buf.Partial
	msg.Timestamp = time.Unix(0, buf.TimeNano)
	err := l.Log(&msg)
	if err != nil {
		logrus.WithField("id", containerid).WithError(err).WithField("message", msg).Error("error writing log message")
		return false
	}
	return true
}

// New 返回特定类型的Logging Driver
// 支持的驱动类型:
// graylog - 目前仅支持udp协议
func New(info logger.Info) (logger.Logger, error) {
	switch strings.ToLower(strings.TrimSpace(info.Config["driver"])) {
	case "graylog":
		//p, _ := strconv.Atoi(info.Config["port"])
		//lcg := LCGrayLog{
		//	URL:      info.Config["url"],
		//	Protocal: info.Config["protocal"],
		//	Port:     p,
		//}
		//return &lcg, nil
		return NewGelf(info)
	default:
		//return new(LCGrayLog), nil
		return NewGelf(info)
	}
}
