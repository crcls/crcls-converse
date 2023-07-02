package channel

import (
	"context"
	"crcls-converse/datastore"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"encoding/json"
	"strconv"
	"time"

	"github.com/libp2p/go-libp2p/core/host"

	ipfsDs "github.com/ipfs/go-datastore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var log = logger.GetLogger()

type Message struct {
	Message   string `json:"message"`
	SenderID  string `json:"sender"`
	Timestamp int64  `json:"timestamp"`
}

type Channel struct {
	ctx   context.Context
	io    *inout.IO
	ds    *datastore.Datastore
	key   ipfsDs.Key
	ID    string
	Sub   *pubsub.Subscription
	Topic *pubsub.Topic
	Host  host.Host
}

func (ch *Channel) Publish(message string) error {
	// TODO: encrypt the message using LitProtocol

	ts := time.Now().UnixMicro()
	m := Message{
		Message:   message,
		SenderID:  ch.Host.ID().Pretty(),
		Timestamp: ts,
	}

	msgBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}

	// Append the timestamp
	key := ch.key.Instance(strconv.FormatInt(ts, 10))

	if err := ch.ds.Put(ch.ctx, key, msgBytes); err != nil {
		inout.EmitChannelError(err)
	}

	return ch.Topic.Publish(ch.ctx, msgBytes)
}

// func (ch *Channel) GetRecentMessages(to time.Duration) ([]Message, error) {
// }

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
