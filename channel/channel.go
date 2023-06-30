package channel

import (
	"context"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/host"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var log = logger.GetLogger()

type Channel struct {
	ctx   context.Context
	io    *inout.IO
	ID    string
	Sub   *pubsub.Subscription
	Topic *pubsub.Topic
	Host  host.Host
}

type Message struct {
	Message  string `json:"message"`
	SenderID string `json:"sender"`
}

func (ch *Channel) Publish(message string) error {
	// TODO: encrypt the message using LitProtocol

	m := Message{
		Message:  message,
		SenderID: ch.Host.ID().Pretty(),
	}

	msgBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return ch.Topic.Publish(ch.ctx, msgBytes)
}

func (ch *Channel) Listen() {
	for {
		response, err := ch.Sub.Next(ch.ctx)
		if err != nil {
			inout.EmitChannelError(err)
			return
		}
		// only forward messages delivered by others
		if response.ReceivedFrom == ch.Host.ID() {
			continue
		}

		message := Message{}
		if err := json.Unmarshal(response.Data, &message); err != nil {
			inout.EmitChannelError(err)
		}

		data, err := json.Marshal(&inout.ReplyMessage{
			Type:    "reply",
			Channel: ch.ID,
			Sender:  message.SenderID,
			Message: message.Message,
		})

		if err != nil {
			inout.EmitChannelError(err)
		}

		ch.io.Write(data)
	}
}
