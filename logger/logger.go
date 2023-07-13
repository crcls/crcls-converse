package logger

import (
	"fmt"
	"os"
	"path/filepath"

	logging "github.com/ipfs/go-log/v2"
)

func SetLogLevel(level string) {
	logging.SetLogLevel("crcls", level)
}

func GetLogger() *logging.ZapEventLogger {
	return logging.Logger("crcls")
}

func SetLogFile(logFile string) {
	dir := filepath.Dir(logFile)

	// Create directory if it does not exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			fmt.Printf("Failed to create directory: %v", err)
			os.Exit(0)
		}
	}

	// Touch the file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to touch file: %v", err)
		os.Exit(1)
	}
	file.Close()

	var config = logging.GetConfig()
	config.File = logFile
	logging.SetupLogging(config)
}
