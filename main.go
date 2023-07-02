package main

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/channel"
	"crcls-converse/datastore"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crcls-converse/network"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var (
	portFlag    = flag.Int("p", 0, "PORT to connect on. 3123-3130")
	verboseFlag = flag.Bool("verbose", false, "Verbose output")
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
	io := inout.Connect()

	net := network.New(*portFlag)
	net.Connect(ctx, a)

	ds := datastore.NewDatastore(ctx, net)
	log.Debugf("%+v", ds.Stats())

	chMgr := channel.NewManager(ctx, net.Host, io, ds)

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
			switch cmd.Type {
			case inout.LIST:
				subcmd, err := cmd.NextSubcommand()
				if err != nil {
					log.Fatal(err)
				}

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
				case inout.MESSAGES:
					if chMgr.Active == nil {
						inout.EmitChannelError(fmt.Errorf("No active channel."))
					} else {
						sscmd, err := cmd.NextSubcommand()
						if err != nil {
							log.Fatal(err)
						}
						days, err := strconv.ParseInt(string(sscmd), 10, 64)
						if err != nil {
							log.Fatal(err)
						}
						dur := time.Hour * time.Duration(24*days)
						chMgr.Active.GetRecentMessages(dur)
					}
				}
			case inout.JOIN:
				chid, err := cmd.NextSubcommand()
				if err != nil {
					log.Fatal(err)
				}

				chMgr.Join(string(chid))
			case inout.REPLY:
				if chMgr.Active == nil {
					inout.EmitChannelError(fmt.Errorf("No active channel."))
				} else {
					chMgr.Active.Publish(string(cmd.Data))
				}
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
