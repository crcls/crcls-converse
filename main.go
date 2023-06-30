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

	io := inout.Connect()

	chMgr := channel.NewManager(ctx, net.Host, io)

	readyEvent, err := json.Marshal(&inout.ReadyMessage{Type: "ready", Status: "connected", Host: net.Host.ID(), PeerCount: int64(len(net.Peers))})
	if err != nil {
		log.Fatal(err)
	}
	io.Write(readyEvent)

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
					data, err := json.Marshal(&inout.ListChannelsMessage{
						Type:     "List",
						Subject:  subcmd,
						Channels: chMgr.ListChannels(),
					})
					if err != nil {
						log.Fatal(err)
					}

					io.Write(data)
				case inout.PEERS:
					peers, err := json.Marshal(&inout.ListPeersMessage{
						Type:    "list",
						Subject: subcmd,
						Peers:   net.Peers,
					})
					if err != nil {
						log.Fatal(err)
					}

					io.Write(peers)
				}
			case inout.JOIN:
				chid, err := cmd.NextSubcomand()
				if err != nil {
					log.Fatal(err)
				}

				chMgr.Join(string(chid))
			}
		case status := <-net.StatusChan:
			if status.Error != nil {
				log.Fatal(status.Error)
			}

			data, err := json.Marshal(inout.PeerMessage{Type: "peer", Connected: status.Connected, Id: status.Peer.PeerID})
			if err != nil {
				log.Fatal(err)
			}

			io.Write(data)
		case <-stop:
			cancel()
			break
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
