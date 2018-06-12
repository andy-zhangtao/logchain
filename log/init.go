package log

import "github.com/andy-zhangtao/gogather/zlog"

var Z *zlog.Zlog

var TrackID string

const (
	ModuleName = "LogChain"
)

func init() {
	Z = zlog.GetZlog()
}

func MyTrackID(id string) {
	TrackID = id
}
