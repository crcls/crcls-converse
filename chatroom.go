package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// ChatRoomBufSize is the number of incoming messages to buffer for each topic.
const ChatRoomBufSize = 128

// ChatRoom represents a subscription to a single PubSub topic. Messages
// can be published to the topic with ChatRoom.Publish, and received
// messages are pushed to the Messages channel.
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

// ChatMessage gets converted to/from JSON and sent in the body of pubsub messages.
type ChatMessage struct {
	Message    string
	SenderID   string
	SenderNick string
}

// JoinChatRoom tries to subscribe to the PubSub topic for the room name, returning
// a ChatRoom on success.
func JoinChatRoom(ctx context.Context, ps *pubsub.PubSub, selfID peer.ID, name, roomName string, log *logging.ZapEventLogger, errChan chan error) {
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

	log.Infof("You have now joined the %s room.\n", roomName)

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

func handleStream(stream network.Stream) {
	fmt.Println("Got a new stream!")

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

	// 'stream' will stay open until you close it (or the other side closes it).
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}
