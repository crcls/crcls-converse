package channel

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/datastore"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crcls-converse/network"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/peer"

	ipfsDs "github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
)

type ChannelManager struct {
	acc      *account.Account
	ctx      context.Context
	channels map[string]Channel
	io       *inout.IO
	ds       *datastore.Datastore
	net      *network.Network
	Active   *Channel
	Address  common.Address
	log      *logging.ZapEventLogger
}

func NewManager(ctx context.Context, a *account.Account, net *network.Network, io *inout.IO, ds *datastore.Datastore) *ChannelManager {
	log := logger.GetLogger()
	ch := &ChannelManager{
		acc:     a,
		ctx:     ctx,
		net:     net,
		io:      io,
		ds:      ds,
		log:     log,
		Address: a.Wallet.Address,
	}

	return ch
}

func (chm *ChannelManager) Join(id string) {
	var ch Channel
	var ok bool

	if chm.Active != nil && chm.Active.ID == id {
		ch = *chm.Active
	} else {
		for _, channel := range chm.channels {
			channel.IsActive = false
		}

		if ch, ok = chm.channels[id]; !ok {
			// join the pubsub topic
			topic, err := chm.net.PubSub.Join(id)
			if err != nil {
				inout.EmitError(err)
				return
			}

			// and subscribe to it
			sub, err := topic.Subscribe()
			if err != nil {
				inout.EmitError(err)
				return
			}

			ch.Sub = sub

			pubKey, err := chm.Address.MarshalText()
			if err != nil {
				inout.EmitError(err)
				return
			}

			ch = Channel{
				ctx:      chm.ctx,
				io:       chm.io,
				ds:       chm.ds,
				key:      ipfsDs.KeyWithNamespaces([]string{"channels", id}),
				log:      chm.log,
				Address:  string(pubKey),
				ID:       id,
				Topic:    topic,
				Host:     (*chm.net.Host),
				Sub:      sub,
				IsActive: true,
				Unread:   0,
			}

			go ch.ListenMessages()
			go ch.ListenDatastore()
		} else {
			ch.IsActive = true
			ch.Unread = 0
		}
	}

	history, err := ch.GetRecentMessages(time.Hour * 24 * 7)
	if err != nil {
		inout.EmitError(err)
		history = make([]inout.Message, 0)
	}

	peers := chm.ListPeers(ch.ID)

	evt := inout.JoinMessage{
		Type:    "join",
		Channel: ch.ID,
		History: history,
		Members: int64(len(peers)),
	}
	data, err := json.Marshal(evt)
	if err != nil {
		chm.log.Fatal(err)
	}

	ch.io.Write(data)
	chm.Active = &ch
}

func (chm *ChannelManager) ListPeers(id string) []peer.ID {
	return chm.net.PubSub.ListPeers(id)
}

func (chm *ChannelManager) ListChannels() []string {
	// TODO: get the list of channels from storage somewhere

	// log.Debug("Only the global channel is availble, so far.")
	return []string{"/crcls"}
}
