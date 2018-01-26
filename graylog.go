package main

import (
	"github.com/docker/docker/daemon/logger"
	"fmt"
	"github.com/robertkowalski/graylog-golang"
	"encoding/json"
)

// LCGrayLog
// URL graylog服务器地址
// Protocal 协议类型 tcp/udp
// Port 端口号
type LCGrayLog struct {
	URL       string
	Protocal  string
	Port      int
	needClose chan int /*关闭channel*/
}

func (lcg *LCGrayLog) Log(msg *logger.Message) error {
	g := gelf.New(gelf.Config{
		GraylogPort:     lcg.Port,
		GraylogHostname: lcg.URL,
		Connection:      "wan",
		MaxChunkSizeWan: 42,
		MaxChunkSizeLan: 1337,
	})
	//fmt.Printf("GrayLog Send Log [%s] \n", msg.Line)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	g.Log(`{
      "version": "1.0",
      "host": "localhost",
      "timestamp": 1356262644,
      "facility": "Google Go",
      "short_message": "Hello From Golang!"
  }`)
	g.Log(string(data))
	fmt.Printf("GrayLog Send [%s] \n", string(data))
	return nil
}

func (lcg *LCGrayLog) Name() string {
	return "graylog"
}

func (lcg *LCGrayLog) Close() error {
	fmt.Println("GrayLog Closed")
	//lcg.needClose <- 1
	return nil
}
