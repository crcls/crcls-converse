package inout

import "github.com/libp2p/go-libp2p/core/peer"

type ErrorMessage struct {
	message string
}

type StatusMessage struct {
	status string
}

type PeerMessage struct {
	Connected bool    `json:"connected"`
	Id        peer.ID `json:"id"`
}
