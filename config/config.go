package config

import (
	"crcls-converse/logger"
	"flag"
	"os"
	"path"
)

var log = logger.GetLogger()

type Config struct {
	HomeDir  string
	CrclsDir string
	Port     int
}

var (
	portFlag    = flag.Int("p", 0, "PORT to connect on. 3123-3130")
	verboseFlag = flag.Bool("verbose", false, "Verbose output")
)

func New() *Config {
	flag.Parse()

	if *verboseFlag {
		logger.SetLogLevel("debug")
	} else {
		logger.SetLogLevel("info")
	}

	hd, err := os.UserHomeDir()
	if err != nil {
		hd = "/"
	}

	crclsDir := path.Join(hd, ".crcls")

	_, derr := os.Stat(crclsDir)
	if os.IsNotExist(derr) {
		if err := os.MkdirAll(crclsDir, 0755); err != nil {
			log.Fatal(err)
		}
	}

	return &Config{
		HomeDir:  hd,
		CrclsDir: crclsDir,
		Port:     *portFlag,
	}
}
