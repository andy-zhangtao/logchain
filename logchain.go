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
	"github.com/docker/docker/api/types/plugins/logdriver"
	"time"
	"strings"
	"strconv"
	"github.com/Sirupsen/logrus"
	"encoding/binary"
	protoio "github.com/gogo/protobuf/io"
	"github.com/andy-zhangtao/logchain/log"
)

type LogChain struct {
	mu     sync.Mutex
	logs   map[string]*logPair
	idx    map[string]*logPair
	stop   chan int
	logger logger.Logger
}

type logPair struct {
	jsonl    logger.Logger
	driver   logger.Logger
	stream   io.ReadCloser
	info     logger.Info
	bufLines int        /*一次缓存的行数*/
	tempStr  []string   /*缓存的日志*/
	mutex    sync.Mutex /*同步锁防止数据错误*/
}

var bufMap map[string]logdriver.LogEntry
//var tempStr []string

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

	var ts []string
	var mutex sync.Mutex
	lf := &logPair{jsonl, log, f, lr.Info, line, ts, mutex}

	lc.logs[lr.File] = lf
	lc.idx[lr.Info.ContainerID] = lf
	lc.mu.Unlock()

	go consumeLog(lf)
	//go func() {
	//	/*每10秒推送一次日志*/
	//	for {
	//		now := time.Now()
	//		next := now.Add(time.Minute * 1)
	//		next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), next.Minute(), 0, 0, next.Location())
	//		t := time.NewTimer(next.Sub(now))
	//
	//		select {
	//		case <-t.C:
	//			buf := getLogEntry(lr.Info.ContainerID)
	//			if len(lf.tempStr) > 0 || len(buf.Line) > 0 {
	//				lf.mutex.Lock()
	//				buf.Line = append([]byte(strings.Join(lf.tempStr, "\n\r")), buf.Line...)
	//				sendMessage(lf.driver, buf, lf.info.ContainerID)
	//				lf.mutex.Unlock()
	//				lf.resetStr()
	//			}
	//
	//		}
	//	}
	//}()
	go func() {
		select {
		case <-lc.stop:
			buf := getLogEntry(lr.Info.ContainerID)
			buf.Line = append([]byte(strings.Join(lf.tempStr, "\n\r")), buf.Line...)
			sendMessage(lf.driver, buf, lf.info.ContainerID)
		}
	}()
	return nil
}

func (lc *LogChain) HandlerStop(lr logging.LogsRequest) error {
	lc.stop <- 1
	return nil
}

func (lc *LogChain) HandlerRead(config logging.LogsReadRequest) (*logger.LogWatcher, error) {
	lr := lc.idx[config.Info.ContainerID]
	if jsReader, ok := lr.jsonl.(logger.LogReader); !ok {
		return nil, errors.New("Get LogReader Errro")
	} else {
		return jsReader.ReadLogs(config.Config), nil
	}
}

func consumeLog(lf *logPair) {

	dec := protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
	defer dec.Close()
	buf := getLogEntry(lf.info.ContainerID)

	for {

		if err := dec.ReadMsg(buf); err != nil {
			if err == io.EOF {
				fmt.Errorf("Name [%s] err [%s] shutting down log logger \n", lf.info.ContainerName, err.Error())
				if len(lf.tempStr) > 0 {
					buf.Line = append([]byte(strings.Join(lf.tempStr, "\n\r")), buf.Line...)
					sendMessage(lf.driver, buf, lf.info.ContainerID)
				}
				lf.stream.Close()
				return
			}
			dec = protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
		}

		lf.jsonl.Log(&logger.Message{Line: buf.Line, Source: lf.info.ContainerName})

		buf.Line = append([]byte(strings.Join(lf.tempStr, "\n\r")), buf.Line...)
		if sendMessage(lf.driver, buf, lf.info.ContainerID) == false {
			continue
		}

		lf.resetStr()

		buf.Reset()
	}
}

//func consumeLog(lf *logPair) {
//
//	dec := protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
//	defer dec.Close()
//	buf := getLogEntry(lf.info.ContainerID)
//
//	idx := 0
//	for {
//		idx ++
//		if err := dec.ReadMsg(buf); err != nil {
//			if err == io.EOF {
//				fmt.Errorf("Name [%s] err [%s] shutting down log logger \n", lf.info.ContainerName, err.Error())
//				if len(lf.tempStr) > 0 {
//					buf.Line = append([]byte(strings.Join(lf.tempStr, "\n\r")), buf.Line...)
//					sendMessage(lf.driver, buf, lf.info.ContainerID)
//				}
//				lf.stream.Close()
//				return
//			}
//			dec = protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
//		}
//
//		lf.jsonl.Log(&logger.Message{Line: buf.Line, Source: lf.info.ContainerName})
//
//		if idx >= lf.bufLines {
//			buf.Line = append([]byte(strings.Join(lf.tempStr, "\n\r")), buf.Line...)
//			if sendMessage(lf.driver, buf, lf.info.ContainerID) == false {
//				continue
//			}
//			//lf.tempStr = lf.tempStr[:0]
//			lf.resetStr()
//			idx = 0
//		} else {
//			//lf.tempStr = append(lf.tempStr, string(buf.Line))
//			lf.addStr(string(buf.Line))
//		}
//
//		buf.Reset()
//	}
//}

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
	logrus.WithFields(logrus.Fields{"Driver": strings.ToLower(strings.TrimSpace(info.Config["driver"]))}).Info("logchain")
	switch strings.ToLower(strings.TrimSpace(info.Config["driver"])) {
	case "graylog":
		log.MyTrackID(info.Config["_track_id"])
		bufMap = make(map[string]logdriver.LogEntry)
		return NewGelf(info)
	case "influx":
		bufMap = make(map[string]logdriver.LogEntry)
		return NewInfluxLog(info)
	default:
		return jsonfilelog.New(info)
	}
}

//getLogEntry 获取容器唯一的日志数据
//id 容器ID
func getLogEntry(id string) *logdriver.LogEntry {
	buf := bufMap[id]
	if buf.Source == "" {
		buf = logdriver.LogEntry{}
	}
	return &buf
}

func (lf *logPair) addStr(str string) {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()

	lf.tempStr = append(lf.tempStr, str)
}

func (lf *logPair) resetStr() {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()

	lf.tempStr = lf.tempStr[:0]
}
