package chatroom

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// ChatRoomBufSize is the number of incoming messages to buffer for each topic.
const ChatRoomBufSize = 128

type ChatRoom struct {
	// Messages is a channel of messages received from other peers in the chat room
	Messages chan *ChatMessage
	Errors   chan error

	ctx   context.Context
	ps    *pubsub.PubSub
	topic *pubsub.Topic
	sub   *pubsub.Subscription

	roomName string
	self     peer.ID
	name     string
	log      *logging.ZapEventLogger
}

type ChatMessage struct {
	Message    string
	SenderID   string
	SenderNick string
}

func Join(ctx context.Context, selfID peer.ID, name, roomName string, errChan chan error) {
	ps, err := pubsub.NewGossipSub(ctx)
	if err != nil {
		panic(err)
	}

	// join the pubsub topic
	topic, err := ps.Join(roomName)
	if err != nil {
		errChan <- err
	}

	// and subscribe to it
	sub, err := topic.Subscribe()
	if err != nil {
		errChan <- err
	}

	log.Infof("You have now joined the %s room.", roomName)

	cr := &ChatRoom{
		ctx:      ctx,
		ps:       ps,
		topic:    topic,
		sub:      sub,
		self:     selfID,
		name:     name,
		roomName: roomName,
		Messages: make(chan *ChatMessage, ChatRoomBufSize),
		Errors:   errChan,
		log:      log,
	}

	// start reading messages from the subscription in a loop
	go cr.streamConsoleTo()
	go cr.printMessagesFrom()
}

// Publish sends a message to the pubsub topic.
func (cr *ChatRoom) Publish(message string) error {
	m := ChatMessage{
		Message:    message,
		SenderID:   cr.self.Pretty(),
		SenderNick: cr.name,
	}
	cr.log.Debug("%+v\n", m)
	msgBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}

	// TODO: encrypt the message using LitProtocol

	return cr.topic.Publish(cr.ctx, msgBytes)
}

func (cr *ChatRoom) ListPeers() []peer.ID {
	return cr.ps.ListPeers(cr.roomName)
}

func (cr *ChatRoom) streamConsoleTo() {
	reader := bufio.NewReader(os.Stdin)
	for {
		s, err := reader.ReadString('\n')
		if err != nil {
			cr.Errors <- err
			return
		}
		// Publish to the network
		if err := cr.Publish(s); err != nil {
			cr.log.Error("### Publish error:", err)
		}
	}
}

func (cr *ChatRoom) printMessagesFrom() {
	for {
		msg, err := cr.sub.Next(cr.ctx)
		if err != nil {
			cr.Errors <- err
			return
		}
		// only forward messages delivered by others
		if msg.ReceivedFrom == cr.self {
			continue
		}

		cm := ChatMessage{}
		err = json.Unmarshal(msg.Data, &cm)
		if err != nil {
			cr.Errors <- err
			return
		}

		fmt.Printf("%s: %s\n", cm.SenderNick, cm.Message)
		fmt.Printf("%s: ", cr.name)
	}
}
