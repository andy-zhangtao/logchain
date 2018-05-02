package main

import (
	"github.com/docker/docker/daemon/logger"
	"net/url"
	"fmt"
	"github.com/docker/docker/pkg/urlutil"
	"net"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"time"
	"github.com/influxdata/influxdb/client/v2"
)

//Write by zhangtao<ztao8607@gmail.com> . In 2018/4/30.
//Send Log To Our Influxdb Via UDP

type influxLogger struct {
	address *url.URL
	info    logger.Info
	name    string
	//rawExtra json.RawMessage
	tags map[string]string
}

func NewInfluxLog(info logger.Info) (logger.Logger, error) {
	address, err := parseInfluxAddress(info.Config["influx-address"])
	if err != nil {
		return nil, err
	}

	//hostname, err := info.Hostname()
	//if err != nil {
	//	return nil, fmt.Errorf("influx: cannot access hostname to set source field. HostName field lost!")
	//}

	tag, err := loggerutils.ParseLogTag(info, loggerutils.DefaultTemplate)
	if err != nil {
		return nil, err
	}

	extra := map[string]string{
		"_container_id":   info.ContainerID,
		"_container_name": info.Name(),
		"_image_id":       info.ContainerImageID,
		"_image_name":     info.ContainerImageName,
		"_command":        info.Command(),
		"_tag":            tag,
	}

	extraAttrs, err := info.ExtraAttributes(func(key string) string {
		if key[0] == '_' {
			return key
		}
		return "_" + key
	})

	if err != nil {
		return nil, err
	}

	for k, v := range extraAttrs {
		extra[k] = v
	}

	//默认情况下name为容器所使用的Image,如果有_svcname则使用此属性
	name := info.ContainerImageName
	if svcname, ok := extra["_svcname"]; ok {
		name = svcname
	}

	return &influxLogger{
		address: address,
		info:    info,
		name:    name,
		tags:    extra,
	}, nil
}

func parseInfluxAddress(address string) (*url.URL, error) {
	if address == "" {
		return nil, fmt.Errorf("influx-address is a required parameter")
	}
	if !urlutil.IsTransportURL(address) {
		return nil, fmt.Errorf("influx-address should be in form proto://address, got %v", address)
	}
	url, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	// we support only udp
	if url.Scheme != "udp" {
		return nil, fmt.Errorf("influx: endpoint must be UDP")
	}

	// get host and port
	if _, _, err = net.SplitHostPort(url.Host); err != nil {
		return nil, fmt.Errorf("influx: please provide influx-address as proto://host:port")
	}

	return url, nil
}

func (this *influxLogger) Log(msg *logger.Message) error {

	c, err := client.NewUDPClient(client.UDPConfig{
		Addr: this.address.Host,
	})
	if err != nil {
		fmt.Println(fmt.Sprintf("Error creating InfluxDB Client: [%s] Address:[%s]", err.Error(), this.address.Host))
		return err
	}
	defer c.Close()

	bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:         "devex",
		Precision:        "ns",
		WriteConsistency: "all",
	})

	fields := map[string]interface{}{
		"log": string(msg.Line),
	}

	//logrus.WithFields(logrus.Fields{"tags": this.tags}).Info("logchain")
	pt, err := client.NewPoint(this.name, this.tags, fields, time.Now())
	if err != nil {
		fmt.Println("Error: ", err.Error())
		return err
	}

	bp.AddPoint(pt)

	err = c.Write(bp)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	return nil
}

func (this *influxLogger) Close() error {
	return nil
}

func (this *influxLogger) Name() string {
	return "influx"
}
