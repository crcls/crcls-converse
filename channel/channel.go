package channel

import (
	"context"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var log = logger.GetLogger()

type Channel struct {
	ctx   context.Context
	ps    *pubsub.PubSub
	topic *pubsub.Topic
	sub   *pubsub.Subscription
	io    *inout.IO
	name  string
	self  peer.ID
}

type Message struct {
	Message  string `json:"message"`
	SenderID string `json:"sender"`
	Channel  string `json:"channel"`
}

func handleError(err error, channel string, io *inout.IO) {
	data, merr := json.Marshal(&inout.ChannelError{
		Type:    "error",
		Channel: channel,
		Error:   err,
	})
	if merr != nil {
		log.Fatal(merr)
	}

	io.Write(data)
}

func Join(ctx context.Context, h host.Host, channel string, io *inout.IO) *Channel {
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		handleError(err, channel, io)
		return nil
	}

	// join the pubsub topic
	topic, err := ps.Join(channel)
	if err != nil {
		handleError(err, channel, io)
		return nil
	}

	// and subscribe to it
	sub, err := topic.Subscribe()
	if err != nil {
		handleError(err, channel, io)
		return nil
	}

	log.Debugf("Joined: %s", channel)

	ch := &Channel{
		ctx:   ctx,
		ps:    ps,
		topic: topic,
		sub:   sub,
		self:  h.ID(),
		name:  channel,
		io:    io,
	}

	go ch.Listen()

	return ch
}

// Publish sends a message to the pubsub topic.
func (ch *Channel) Publish(message string) error {
	// TODO: encrypt the message using LitProtocol

	m := Message{
		Message:  message,
		SenderID: ch.self.Pretty(),
		Channel:  ch.name,
	}
	log.Debug("%+v\n", m)

	msgBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return ch.topic.Publish(ch.ctx, msgBytes)
}

func (ch *Channel) Listen() {
	for {
		msg, err := ch.sub.Next(ch.ctx)
		if err != nil {
			handleError(err, ch.name, ch.io)
			return
		}
		// only forward messages delivered by others
		if msg.ReceivedFrom == ch.self {
			continue
		}

		data, err := json.Marshal(&inout.ReplyMessage{
			Type:    "reply",
			Channel: ch.name,
			Peer:    string(msg.ReceivedFrom),
			Message: string(msg.Data),
		})

		if err != nil {
			handleError(err, ch.name, ch.io)
		}

		ch.io.Write(data)
	}
}

func (ch *Channel) ListOnlineMembers() []peer.ID {
	return ch.ps.ListPeers(ch.name)
}
