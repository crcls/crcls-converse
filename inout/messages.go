package inout

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Message struct {
	Message   string `json:"message"`
	Sender    string `json:"sender"`
	Timestamp int64  `json:"timestamp"`
}

type ReplyMessage struct {
	Type    string `json:"type"`
	Sender  string `json:"sender"`
	Message string `json:"message"`
}

type JoinMessage struct {
	Type    string    `json:"type"`
	Channel string    `json:"channel"`
	History []Message `json:"history"`
	Members int64     `json:"members"`
}

type StatusMessage struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type PeerMessage struct {
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
	Id        string `json:"id"`
}

type NoAccountMessage struct {
	Type string `json:"type"`
}

type NewAccount struct {
	Type       string         `json:"type"`
	Address    common.Address `json:"address"`
	SeedPhrase string         `json:"seedPhrase"`
	Balance    *big.Int       `json:"balance"`
}

type ListChannelsMessage struct {
	Type     string   `json:"type"`
	Subject  CMD      `json:"subject"`
	Channels []string `json:"channels"`
}

type ListPeersMessage struct {
	Type    string    `json:"type"`
	Subject CMD       `json:"subject"`
	Peers   []peer.ID `json:"peers"`
}

type ListMessagesMessage struct {
	Type     string    `json:"type"`
	Channel  string    `json:"channel"`
	Messages []Message `json:"messages"`
}

type MemberChangeMessage struct {
	Type string `json:"type"`
}

type MemberCreateMessage struct {
	Type    string `json:"type"`
	Handle  string `json:"handle"`
	Address string `json:"address"`
	PFP     string `json:"pfp"`
}

func EmitMessage(msg []byte) {
	internalIO.Write(msg)
}
