# logchain
Docker log plugin. For combine multiple log lines

## How to use?

### Install Plugin

```
docker plugin install vikings/logchain:<version> --alias logchain
```

```
docker plugin enable logchain
```

### Use it in docker run.

```
docker run --log-driver logchain --log-opt buf="10" --log-opt gelf-address=udp://xxxx \
--log-opt env=xxxx
```

### Use it in systemd

Modify docker systemd service
```
#defined $LOG_DRIVER=--log-driver=logchain
ExecStart=/bin/dockerd $LOG_LEVEL $LIVE_RESTORE $LOG_DRIVER $LOG_OPT $BIP $STORAGE_DRIVER $GRAPH $REGISTRY_MIRROR $STORAGE_OPT
```

Create container
```
docker run -e log-opt="--log-opt buf="10";--log-opt gelf-address=udp://xxxx;--log-opt env=xxxx "
```

## Local Dev & Test

Build
```
make rootfs
```

Test (Recommend Test Via Vagrant+Coreos) In This Path
```
docker plugin create logchain $PWD
```

Enable Plugin
```
docker enable logchain
```

Test it!

## Change Log
* v1.0.7
  - 支持定时输出日志. 此版本间隔1分钟刷新一次日志

* v1.0.6
  - 修复输出日志时，不同容器之间相互干扰输出的问题

* v1.0.5
  - 支持通过`docker logs`命令读取日志

* v1.0.4
  - 当容器退出时,按照指定的log driver处理缓存数据，而不是直接丢弃