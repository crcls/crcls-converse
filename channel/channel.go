package channel

import (
	"context"
	"crcls-converse/datastore"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"

	ipfsDs "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var log = logger.GetLogger()

type Channel struct {
	ctx      context.Context
	io       *inout.IO
	ds       *datastore.Datastore
	key      ipfsDs.Key
	ID       string
	Sub      *pubsub.Subscription
	Topic    *pubsub.Topic
	Host     host.Host
	IsActive bool
	Unread   int16
}

func (ch *Channel) Publish(message string) error {
	// TODO: encrypt the message using LitProtocol

	ts := time.Now().UnixMicro()
	m := inout.Message{
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

	// Save the message to the datastore
	return ch.ds.Put(ch.ctx, key, msgBytes)
}

func (ch *Channel) GetRecentMessages(timespan time.Duration) ([]inout.Message, error) {
	prefix := ch.key.Parent()
	startTime := time.Now().Add(-timespan)
	msgs := make([]inout.Message, 0)

	q := query.Query{
		Prefix:   prefix.String(),
		Orders:   []query.Order{query.OrderByKeyDescending{}},
		KeysOnly: false,
	}

	results, err := ch.ds.Query(ch.ctx, q)
	if err != nil {
		log.Debug(err)
		return msgs, err
	}
	defer results.Close()

	entries := make([]query.Result, 0)
	for res := range results.Next() {
		// Extract the key and parse the timestamp
		key := res.Entry.Key
		keyParts := strings.Split(key, ":")
		timestampStr := keyParts[len(keyParts)-1]

		timestampMicro, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			log.Debug(err)
			continue
		}
		timestamp := time.Unix(0, timestampMicro*int64(time.Microsecond))

		if timestamp.After(startTime) {
			entries = append(entries, res)
		}
	}

	for _, entry := range entries {
		reply := inout.ReplyMessage{}
		if err := json.Unmarshal(entry.Value, &reply); err != nil {
			log.Error(err)
			continue
		}

		key := ipfsDs.NewKey(entry.Key)
		base := strings.Split(key.BaseNamespace(), ":")
		ts, err := strconv.ParseInt(base[1], 10, 64)

		if err != nil {
			log.Fatal(err)
		}

		msg := inout.Message{
			Message:   reply.Message,
			SenderID:  reply.Sender,
			Timestamp: ts,
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}

func (ch *Channel) ListenDatastore() {
	for {
		select {
		case entry := <-ch.ds.EventStream:
			if entry.Key.IsDescendantOf(ch.key) {
				if ch.IsActive {
					ch.EmitReply(entry.Value)
				} else {
					ch.Unread += 1
				}
			}
		case <-ch.ctx.Done():
			return
		}
	}
}

func (ch *Channel) ListenMessages() {
	for {
		response, err := ch.Sub.Next(ch.ctx)
		if err != nil {
			inout.EmitError(err)
			return
		}
		// only forward messages delivered by others
		if response.ReceivedFrom == ch.Host.ID() {
			continue
		}

		log.Debugf("Topic message: %+v\n", response)

		// TODO: Decide what the open topic PubSub should be used for.
	}
}

func (ch *Channel) EmitReply(msg []byte) {
	message := inout.Message{}
	if err := json.Unmarshal(msg, &message); err != nil {
		inout.EmitError(err)
	}

	data, err := json.Marshal(&inout.ReplyMessage{
		Type:    "reply",
		Sender:  message.SenderID,
		Message: message.Message,
	})

	if err != nil {
		inout.EmitError(err)
	}

	ch.io.Write(data)
}
