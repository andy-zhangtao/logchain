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
