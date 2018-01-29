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
	"syscall"
	"github.com/tonistiigi/fifo"
	"context"
	"encoding/binary"
	protoio "github.com/gogo/protobuf/io"
	"github.com/docker/docker/api/types/plugins/logdriver"
	"time"
	"strings"
	"strconv"
)

type LogChain struct {
	mu     sync.Mutex
	logs   map[string]*logPair
	idx    map[string]*logPair
	logger logger.Logger
}

type logPair struct {
	jsonl    logger.Logger
	driver   logger.Logger
	stream   io.ReadCloser
	info     logger.Info
	bufLines int /*一次缓存的行数*/
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

	f, err := fifo.OpenFifo(context.Background(), lr.File, syscall.O_RDONLY, 0700)
	if err != nil {
		return errors.Wrapf(err, "error opening logger fifo: %q", lr.File)
	}

	lc.mu.Lock()

	line, err := strconv.Atoi(lr.Info.Config["buf"])
	if err != nil {
		line = 1
	}

	lf := &logPair{jsonl, log, f, lr.Info, line}

	lc.logs[lr.File] = lf
	lc.idx[lr.Info.ContainerID] = lf
	lc.mu.Unlock()

	go consumeLog(lf)

	return nil
}

func (lc *LogChain) HandlerStop(logging.LogsRequest) error {
	//fmt.Println("======handler stop")
	return nil
}

func consumeLog(lf *logPair) {
	var tempStr []string

	dec := protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
	defer dec.Close()
	var buf logdriver.LogEntry
	idx := 0
	for {
		idx ++
		if err := dec.ReadMsg(&buf); err != nil {
			if err == io.EOF {
				fmt.Errorf("id [%s] err [%s] shutting down log logger \n", lf.info.ContainerID, err.Error())
				if len(tempStr) > 0 {
					buf.Line = append([]byte(strings.Join(tempStr, "\n\r")), buf.Line...)
					sendMessage(lf.driver, &buf, lf.info.ContainerID)
				}
				lf.stream.Close()
				return
			}
			dec = protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
		}

		if idx >= lf.bufLines {
			buf.Line = append([]byte(strings.Join(tempStr, "\n\r")), buf.Line...)
			if sendMessage(lf.driver, &buf, lf.info.ContainerID) == false {
				continue
			}
			tempStr = tempStr[:0]
			idx = 0
		} else {
			tempStr = append(tempStr, string(buf.Line))
		}

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
		fmt.Errorf("id [%s] err [%s] error writing log message \n", containerid, err.Error())
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
		return NewGelf(info)
	default:
		return NewGelf(info)
	}
}
