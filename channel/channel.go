package channel

import (
	"context"
	"crcls-converse/datastore"
	"crcls-converse/inout"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"

	ipfsDs "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type Channel struct {
	ctx      context.Context
	io       *inout.IO
	ds       *datastore.Datastore
	key      ipfsDs.Key
	ID       string
	Address  string
	Sub      *pubsub.Subscription
	Topic    *pubsub.Topic
	Host     host.Host
	IsActive bool
	Unread   int16
	log      *logging.ZapEventLogger
}

func (ch *Channel) Publish(message string) error {
	// TODO: encrypt the message using LitProtocol

	ts := time.Now().UnixMicro()
	m := inout.Message{
		Message:   message,
		Sender:    ch.Address,
		Timestamp: ts,
	}

	msgBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}

	// Append the timestamp
	key := ch.key.ChildString(strconv.FormatInt(ts, 10)).Instance(string(ch.Address))

	// Save the message to the network
	if err = ch.ds.Put(ch.ctx, key, msgBytes); err != nil {
		return err
	}

	msg := inout.StatusMessage{
		Type:   "replyStatus",
		Status: "sent",
	}

	data, err := json.Marshal(&msg)
	if err != nil {
		ch.log.Fatal(data)
	}

	ch.io.Write(data)

	return nil
}

func (ch *Channel) GetRecentMessages(timespan time.Duration) ([]inout.Message, error) {
	prefix := ch.key
	startTime := time.Now().Add(-timespan)
	msgs := make([]inout.Message, 0)

	q := query.Query{
		Filters:  []query.Filter{query.FilterKeyPrefix{Prefix: prefix.String()}},
		KeysOnly: false,
	}

	results, err := ch.ds.Query(ch.ctx, q)
	if err != nil {
		ch.log.Debug(err)
		return msgs, err
	}
	defer results.Close()

	entries := make([]query.Result, 0)
	for res := range results.Next() {
		// Extract the key and parse the timestamp
		key := ipfsDs.NewKey(res.Entry.Key)
		keyParts := strings.Split(key.BaseNamespace(), ":")
		timestampStr := keyParts[0]

		timestampMicro, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			ch.log.Debug(err)
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
			ch.log.Error(err)
			continue
		}

		key := ipfsDs.NewKey(entry.Key)
		base := strings.Split(key.BaseNamespace(), ":")
		ts, err := strconv.ParseInt(base[0], 10, 64)

		if err != nil {
			ch.log.Fatal(err)
		}

		msg := inout.Message{
			Message:   reply.Message,
			Sender:    reply.Sender,
			Timestamp: ts,
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}

func (ch *Channel) ListenDatastore() {
	msgChan := ch.ds.Subscribe(ch.key)
	for {
		select {
		case msg := <-msgChan:
			if ch.IsActive {
				ch.EmitReply(msg)
			} else {
				ch.Unread += 1
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

		ch.log.Debugf("Topic message: %+v\n", response)

		// TODO: Decide what the open topic PubSub should be used for.
		// 1. Requests for data not found in the local DB.
	}
}

func (ch *Channel) EmitReply(msg *inout.Message) {
	data, err := json.Marshal(&inout.ReplyMessage{
		Type:    "reply",
		Sender:  msg.Sender,
		Message: msg.Message,
	})

	if err != nil {
		inout.EmitError(err)
	}

	ch.io.Write(data)
}
