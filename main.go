package main

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/channel"
	"crcls-converse/config"
	"crcls-converse/datastore"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crcls-converse/network"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logger.GetLogger()

type ReadyMessage struct {
	Type   string          `json:"type"`
	Status string          `json:"status"`
	Host   peer.ID         `json:"host"`
	Member *account.Member `json:"member"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	conf := config.New()
	io := inout.Connect()

	a := account.Load(conf, io)
	netw := network.New(conf)
	netw.Connect(ctx, a)
	ds := datastore.NewDatastore(ctx, netw.Host, netw.PubSub, netw.DHT)
	chMgr := channel.NewManager(ctx, netw, io, ds)

	if a.Wallet != nil {
		balance, err := a.Wallet.Balance()
		if err != nil {
			log.Fatal(err)
		}

		log.Debugf("\n\n-----------------------\nWallet Connected\n-----------------------\nAddress: %s\nSeed Phrase: %s\nBalance: %d\n-----------------------\n", a.Wallet.Address, a.Wallet.SeedPhrase, balance)

		ctxTO, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		member, err := account.GetMember(ctxTO, a.Wallet.Address)
		if err != nil {
			log.Debug(err)
		}
		a.Member = member
	}

	readyEvent, err := json.Marshal(&ReadyMessage{Type: "ready", Status: "connected", Host: netw.Host.ID(), Member: a.Member})
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
					inout.EmitError(err)
					continue
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
						Peers:   netw.Peers,
					})
					if err != nil {
						log.Fatal(err)
					}

					io.Write(peers)
				case inout.MESSAGES:
					if chMgr.Active == nil {
						inout.EmitError(fmt.Errorf("No active channel."))
					} else {
						sscmd, err := cmd.NextSubcommand()
						if err != nil {
							inout.EmitError(err)
						} else {
							days, err := strconv.ParseInt(string(sscmd), 10, 64)
							if err != nil {
								log.Fatal(err)
							}
							dur := time.Hour * time.Duration(24*days)
							chMgr.Active.GetRecentMessages(dur)
						}
					}
				}
			case inout.JOIN:
				chid, err := cmd.NextSubcommand()
				if err != nil {
					inout.EmitError(err)
				} else {
					chMgr.Join(string(chid))
				}
			case inout.REPLY:
				if chMgr.Active == nil {
					inout.EmitError(fmt.Errorf("No active channel."))
				} else {
					chMgr.Active.Publish(string(cmd.Data))
				}
			case inout.MEMBER:
				subcmd, err := cmd.NextSubcommand()
				if err != nil {
					inout.EmitError(err)
					continue
				}

				switch subcmd {
				case inout.CREATE:
					if a.Wallet != nil {
						inout.EmitError(fmt.Errorf("Already joined."))
						continue
					}

					member := &account.Member{}
					if err := json.Unmarshal(cmd.Data, member); err != nil {
						inout.EmitError(err)
						continue
					}

					a.CreateWallet()

					prettyAddress, err := a.Wallet.Address.MarshalText()
					if err != nil {
						panic(err)
					}
					fmt.Printf("\nMain: Pretty Address: %v\n\n", string(prettyAddress))

					if err := account.NewMember(ctx, a, member); err != nil {
						inout.EmitError(err)
						continue
					}

					evt := inout.MemberChangeMessage{Type: "memberChange"}
					data, err := json.Marshal(evt)
					if err != nil {
						log.Fatal(err)
					}
					io.Write(data)
				}
			}
		case status := <-netw.StatusChan:
			if status.Error != nil {
				inout.EmitError(status.Error)
			} else {
				data, err := json.Marshal(inout.PeerMessage{Type: "peer", Connected: status.Connected, Id: status.Peer.PeerID})
				if err != nil {
					log.Fatal(err)
				}

				io.Write(data)
			}
		case <-stop:
			cancel()
			break
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
