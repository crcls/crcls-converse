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
	"strconv"
	"time"
)

var crcls *CRCLS

type CRCLS struct {
	Account *account.Account
	ChMgr   *channel.ChannelManager
	Config  *config.Config
	DS      *datastore.Datastore
	Net     *network.Network
	IO      *inout.IO
}

func (crcls *CRCLS) initServices(ctx context.Context) error {
	netw, err := network.New(ctx, crcls.Config, crcls.Account.Wallet.PrivKey)
	if err != nil {
		return err
	}

	ds := datastore.NewDatastore(ctx, crcls.Config, netw.Host, netw.PubSub, netw.DHT)
	ds.Authenticate(crcls.Account.Wallet.PrivKey)

	chMgr := channel.NewManager(ctx, crcls.Account, netw, crcls.IO, ds)

	crcls.Net = netw
	crcls.DS = ds
	crcls.ChMgr = chMgr

	return nil
}

func LoadCRCLS(ctx context.Context, conf *config.Config, io *inout.IO) error {
	a, err := account.Load(conf)
	if err != nil {
		return err
	}

	crcls = &CRCLS{
		Account: a,
		Config:  conf,
		IO:      io,
	}

	if err := crcls.initServices(ctx); err != nil {
		return err
	}

	return nil
}

func CreateCRCLS(ctx context.Context, conf *config.Config, io *inout.IO) error {
	a, err := account.New(conf)
	if err != nil {
		return err
	}

	crcls = &CRCLS{
		Account: a,
		Config:  conf,
		IO:      io,
	}

	if err := crcls.initServices(ctx); err != nil {
		return err
	}

	return nil
}

func (crcls *CRCLS) Run(ctx context.Context, cmd *inout.InputCommand) error {
	ensureInit()
	log := logger.GetLogger()

	switch cmd.Type {
	case inout.LIST:
		subcmd, err := cmd.NextSubcommand()
		if err != nil {
			return err
		}

		switch subcmd {
		case inout.CHANNELS:
			data, err := json.Marshal(&inout.ListChannelsMessage{
				Type:     "list-channels",
				Subject:  subcmd,
				Channels: crcls.ChMgr.ListChannels(),
			})
			if err != nil {
				log.Fatal(err)
			}

			crcls.IO.Write(data)
		case inout.PEERS:
			peers, err := json.Marshal(&inout.ListPeersMessage{
				Type:    "list-peers",
				Subject: subcmd,
				Peers:   crcls.Net.Peers,
			})
			if err != nil {
				log.Fatal(err)
			}

			crcls.IO.Write(peers)
		case inout.MESSAGES:
			if crcls.ChMgr.Active == nil {
				return fmt.Errorf("No active channel.")
			} else {
				sscmd, err := cmd.NextSubcommand()
				if err != nil {
					return err
				} else {
					days, err := strconv.ParseInt(string(sscmd), 10, 64)
					if err != nil {
						log.Fatal(err)
					}
					dur := time.Hour * time.Duration(24*days)
					result, err := crcls.ChMgr.Active.GetRecentMessages(dur)
					if err != nil {
						return err
					}

					messages, err := json.Marshal(&inout.ListMessagesMessage{Type: "list-messages", Channel: crcls.ChMgr.Active.ID, Messages: result})
					if err != nil {
						log.Fatal(err)
					}

					crcls.IO.Write(messages)
				}
			}
		}
	case inout.JOIN:
		chid, err := cmd.NextSubcommand()
		if err != nil {
			return err
		} else {
			crcls.ChMgr.Join(string(chid))
		}
	case inout.REPLY:
		if crcls.ChMgr.Active == nil {
			inout.EmitError(fmt.Errorf("No active channel."))
		} else {
			if err := crcls.ChMgr.Active.Publish(string(cmd.Data)); err != nil {
				inout.EmitError(err)
			}
		}
	case inout.MEMBER:
		ensureInit()
		subcmd, err := cmd.NextSubcommand()
		if err != nil {
			return err
		}

		switch subcmd {
		case inout.CREATE:
			member := &account.Member{}
			if err := json.Unmarshal(cmd.Data, member); err != nil {
				return err
			}

			if err := account.NewMember(ctx, crcls.Account, member, crcls.DS); err != nil {
				inout.EmitError(err)
				// continue
			}

			evt := inout.MemberChangeMessage{Type: "member-create"}
			data, err := json.Marshal(evt)
			if err != nil {
				log.Fatal(err)
			}
			crcls.IO.Write(data)
		}
	}

	return nil
}
