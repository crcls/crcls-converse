package inout

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

type Message struct {
	Message   string `json:"message"`
	SenderID  string `json:"sender"`
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
	Type      string  `json:"type"`
	Connected bool    `json:"connected"`
	Id        peer.ID `json:"id"`
}

type NoAccountMessage struct {
	Type string `json:"type"`
}

type ListChannelsMessage struct {
	Type     string   `json:"type"`
	Subject  CMD      `json:"subject"`
	Channels []string `json:"channels"`
}

type ListPeersMessage struct {
	Type    string             `json:"type"`
	Subject CMD                `json:"subject"`
	Peers   []*peer.PeerRecord `json:"peers"`
}

type MemberChangeMessage struct {
	Type string `json:"type"`
}
