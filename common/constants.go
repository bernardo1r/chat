package common

import "time"

const (
	NameWait   = time.Second * 10
	WriteWait  = time.Second * 10
	PongWait   = time.Second * 60
	PingPeriod = time.Second * 45
)
