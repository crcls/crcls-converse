package main

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/channel"
	"crcls-converse/config"
	"crcls-converse/datastore"
	"crcls-converse/evm"
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

type ReadyMessage struct {
	Type   string          `json:"type"`
	Status string          `json:"status"`
	Host   peer.ID         `json:"host"`
	Member *account.Member `json:"member"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	conf := config.New()
	log := logger.GetLogger()
	io := inout.Connect()

	a := account.Load(conf, io)
	netw := network.New(conf)
	netw.Connect(ctx, a)
	ds := datastore.NewDatastore(ctx, conf, netw.Host, netw.PubSub, netw.DHT)
	chMgr := channel.NewManager(ctx, netw, io, ds)

	if a.Wallet != nil {
		balance, err := a.Wallet.Balance()
		if err != nil {
			log.Fatal(err)
		}

		ctxTO, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		member, err := account.GetMember(ctxTO, a.Wallet.Address, ds)
		if err != nil {
			log.Debug(err)
		} else {
			member.Balance = balance
			a.Member = member
		}
	}

	readyEvent, err := json.Marshal(&ReadyMessage{Type: "ready", Status: "connected", Host: (*netw.Host).ID(), Member: a.Member})
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
			case inout.READY:
				readyEvent, err := json.Marshal(&ReadyMessage{Type: "ready", Status: "connected", Host: (*netw.Host).ID(), Member: a.Member})
				if err != nil {
					log.Fatal(err)
				}
				io.Write(readyEvent)
			case inout.ACCOUNT:
				subcmd, err := cmd.NextSubcommand()
				if err != nil {
					inout.EmitError(err)
					continue
				}

				switch subcmd {
				case inout.CREATE:
					emitNewAccount := func(wallet *evm.Wallet) {
						balance, err := wallet.Balance()
						if err != nil {
							inout.EmitError(err)
							return
						}

						msg := inout.NewAccount{
							Type:       "account-create",
							Address:    wallet.Address,
							SeedPhrase: wallet.SeedPhrase,
							Balance:    balance,
						}

						data, err := json.Marshal(&msg)
						if err != nil {
							inout.EmitError(err)
							return
						}

						io.Write(data)
					}

					if a.Wallet != nil {
						emitNewAccount(a.Wallet)
						continue
					}

					a.CreateWallet()
					emitNewAccount(a.Wallet)
				}
			case inout.LIST:
				subcmd, err := cmd.NextSubcommand()
				if err != nil {
					inout.EmitError(err)
					continue
				}

				switch subcmd {
				case inout.CHANNELS:
					data, err := json.Marshal(&inout.ListChannelsMessage{
						Type:     "list-channels",
						Subject:  subcmd,
						Channels: chMgr.ListChannels(),
					})
					if err != nil {
						log.Fatal(err)
					}

					io.Write(data)
				case inout.PEERS:
					peers, err := json.Marshal(&inout.ListPeersMessage{
						Type:    "list-peers",
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
							// TODO: emit result
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
					member := &account.Member{}
					if err := json.Unmarshal(cmd.Data, member); err != nil {
						inout.EmitError(err)
						continue
					}

					if err := account.NewMember(ctx, a, member, ds); err != nil {
						inout.EmitError(err)
						// continue
					}

					evt := inout.MemberChangeMessage{Type: "member-create"}
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
