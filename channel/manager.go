package channel

import (
	"context"
	"crcls-converse/datastore"
	"crcls-converse/inout"
	"crcls-converse/network"
	"encoding/json"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	ipfsDs "github.com/ipfs/go-datastore"
)

type ChannelManager struct {
	ctx      context.Context
	channels map[string]Channel
	io       *inout.IO
	ds       *datastore.Datastore
	net      *network.Network
	Active   *Channel
}

func NewManager(ctx context.Context, net *network.Network, io *inout.IO, ds *datastore.Datastore) *ChannelManager {
	ch := &ChannelManager{
		ctx: ctx,
		net: net,
		io:  io,
		ds:  ds,
	}

	return ch
}

func (chm *ChannelManager) Join(id string) {
	var ch Channel
	var ok bool

	found := false
	for _, chanId := range chm.ListChannels() {
		if id == chanId {
			found = true
			break
		}
	}

	if !found {
		inout.EmitError(fmt.Errorf("Channel not found. %s", id))
		return
	}

	for _, value := range chm.channels {
		value.IsActive = false
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

		ch = Channel{
			ctx:      chm.ctx,
			io:       chm.io,
			ds:       chm.ds,
			key:      ipfsDs.KeyWithNamespaces([]string{"channels", id, chm.net.Host.ID().Pretty()}),
			ID:       id,
			Topic:    topic,
			Host:     chm.net.Host,
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
		log.Fatal(err)
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
	return []string{"global"}
}
