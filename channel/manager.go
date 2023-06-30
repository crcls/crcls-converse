package channel

import (
	"context"
	"crcls-converse/inout"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type ChannelManager struct {
	ctx      context.Context
	channels map[string]Channel
	io       *inout.IO
	host     host.Host
	ps       *pubsub.PubSub
	Active   Channel
}

func NewManager(ctx context.Context, h host.Host, io *inout.IO) *ChannelManager {
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		inout.EmitChannelError(err)
		return nil
	}

	ch := &ChannelManager{
		ctx:  ctx,
		ps:   ps,
		host: h,
		io:   io,
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
		inout.EmitChannelError(fmt.Errorf("Channel not found. %s", id))
		return
	}

	if ch, ok = chm.channels[id]; !ok {
		// join the pubsub topic
		topic, err := chm.ps.Join(id)
		if err != nil {
			inout.EmitChannelError(err)
			return
		}

		// and subscribe to it
		sub, err := topic.Subscribe()
		if err != nil {
			inout.EmitChannelError(err)
			return
		}

		ch.Sub = sub

		ch = Channel{
			ctx:   chm.ctx,
			io:    chm.io,
			ID:    id,
			Topic: topic,
			Host:  chm.host,
			Sub:   sub,
		}

		go ch.Listen()
	}

	chm.Active = ch
}

func (chm *ChannelManager) ListPeers(id string) []peer.ID {
	return chm.ps.ListPeers(id)
}

func (chm *ChannelManager) ListChannels() []string {
	// TODO: get the list of channels from storage somewhere

	// log.Debug("Only the global channel is availble, so far.")
	return []string{"global"}
}
