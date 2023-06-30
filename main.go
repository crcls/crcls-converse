package main

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/channel"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crcls-converse/network"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	portFlag    = flag.Int("p", 0, "PORT to connect on. 3123-3130")
	verboseFlag = flag.Bool("v", false, "Verbose output")
)

var log = logger.GetLogger()

type ReadyEvent struct {
	Status    string  `json:"status"`
	Host      peer.ID `json:"host"`
	PeerCount int64   `json:"peerCount"`
}

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

	io := inout.Connect()

	readyEvent, err := json.Marshal(&ReadyEvent{Status: "connected", Host: net.Host.ID(), PeerCount: int64(len(net.Peers))})
	if err != nil {
		log.Fatal(err)
	}

	io.Write("ready", readyEvent)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	for {
		select {
		case cmd := <-io.InputChan:
			log.Debugf("Received command: %s with data: %s", cmd.Type, string(cmd.Data))
			// TODO: delegate the command

			switch cmd.Type {
			case inout.LIST:
				subcmd, err := cmd.NextSubcomand()
				if err != nil {
					log.Fatal(err)
				}

				log.Debug(subcmd)

				switch subcmd {
				case inout.CHANNELS:
					channels, err := json.Marshal(channel.List())
					if err != nil {
						log.Fatal(err)
					}

					io.Write("list", channels)
				case inout.PEERS:
					peers, err := json.Marshal(net.Peers)
					if err != nil {
						log.Fatal(err)
					}

					io.Write("list", peers)
				}
			}
		case status := <-net.StatusChan:
			if status.Error != nil {
				log.Fatal(status.Error)
			}

			data, err := json.Marshal(inout.PeerMessage{Connected: status.Connected, Id: status.Peer.PeerID})
			if err != nil {
				log.Fatal(err)
			}

			io.Write("network", data)
		case <-stop:
			cancel()
			break
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
