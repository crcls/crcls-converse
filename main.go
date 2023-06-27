package main

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/logger"
	"crcls-converse/network"
	"flag"
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

	statusChan := network.Connect(ctx, *portFlag, a)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	select {
	case status := <-statusChan:
		log.Infof("Status Event %+v", status)
		// room := fmt.Sprintf("crcls-%s", *roomFlag)
		// log.Debugf("Joining the chat room: %s", room)
		// // create a new PubSub service using the GossipSub router
		// errors := make(chan error, 1)
		// chatroom.Join(ctx, ps, h.ID(), *nameFlag, room, log, errors)

		select {
		// case err := <-errors:
		// 	fmt.Println(err)
		// 	cancel()
		// 	os.Exit(1)
		case <-stop:
			cancel()
		}
	case <-stop:
		cancel()
	case <-ctx.Done():
		os.Exit(0)
	}
}
