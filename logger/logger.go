package logger

import logging "github.com/ipfs/go-log/v2"

var log = logging.Logger("crcls")

func SetLogLevel(level string) {
	logging.SetLogLevel("crcls", level)
}

func GetLogger() *logging.ZapEventLogger {
	return log
}
