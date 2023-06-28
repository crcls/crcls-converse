package main

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crcls-converse/network"
	"flag"
	"os"
	"os/signal"
	"syscall"
)

var (
	portFlag    = flag.Int("p", 0, "PORT to connect on. 3123-3130")
	verboseFlag = flag.Bool("v", false, "Verbose output")
)

var log = logger.GetLogger()

func main() {
	flag.Parse()

	if *verboseFlag {
		logger.SetLogLevel("debug")
	} else {
		logger.SetLogLevel("info")
	}

	ctx, cancel := context.WithCancel(context.Background())

	a := account.New()

	net := network.New(*portFlag)
	net.Connect(ctx, a)
	inout.Connect()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	for {
		select {
		case status := <-net.StatusChan:
			if status.Error != nil {
				log.Fatal(status.Error)
			}

			inout.Write(inout.PeerMessage{Connected: status.Connected, Id: status.Peer.PeerID})
		case <-stop:
			cancel()
			break
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
