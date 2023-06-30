package inout

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

type ErrorMessage struct {
	Type    string `json:"type"`
	message string
}

type ChannelError struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Error   error  `json:"error"`
}

type ReplyMessage struct {
	Type    string `json:"type"`
	Peer    string `json:"peer"`
	Channel string `json:"channel"`
	Message string `json:"message"`
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

type ReadyMessage struct {
	Type      string  `json:"type"`
	Status    string  `json:"status"`
	Host      peer.ID `json:"host"`
	PeerCount int64   `json:"peerCount"`
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
