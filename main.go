package main

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/logger"
	"crcls-converse/network"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	roomFlag     = flag.String("room", "global", "name of topic to join")
	nameFlag     = flag.String("name", "", "Name to use in chat. will be generated if empty")
	relayFlag    = flag.Bool("relay", false, "Enable relay mode for this node.")
	useRelayFlag = flag.String("r", "", "AddrInfo Use the relay node to bypass NAT/Firewalls")
	portFlag     = flag.Int("p", 0, "PORT to connect on. 3123-3130")
	isLeaderFlag = flag.Bool("leader", false, "Start this node in leader mode.")
	verboseFlag  = flag.Bool("v", false, "Verbose output")
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
	statusChan := net.Connect(ctx, a)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	for {
		select {
		case status := <-statusChan:
			b, err := json.Marshal(status)

			if err != nil {
				log.Debugf("Failed to marshal: %+v", status)
			}

			fmt.Printf("%v\n", string(b))
		case <-stop:
			cancel()
			break
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
