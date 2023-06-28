package network

import (
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

type Notifee struct {
	net *Network
}

func (n *Notifee) Listen(network.Network, ma.Multiaddr) {
	// log.Debug("Listen called")
}

func (n *Notifee) ListenClose(network.Network, ma.Multiaddr) {
	// log.Debug("ListenClose called")
}

func (n *Notifee) Connected(net network.Network, con network.Conn) {
	// log.Debugf("Connected called %v", peerRecord)
}

func (n *Notifee) Disconnected(net network.Network, con network.Conn) {
	peerRecord := net.Peerstore().PeerInfo(con.RemotePeer())

	for i, peer := range n.net.Peers {
		if peer.PeerID == peerRecord.ID {
			log.Debugf("Peer %s has disconnected", peerRecord.ID)
			if i < len(n.net.Peers)-1 {
				copy(n.net.Peers[i:], n.net.Peers[i+1:])
			}
			n.net.Peers = n.net.Peers[:len(n.net.Peers)-1]
		}
	}
}

func (n *Notifee) OpenedStream(network.Network, network.Stream) {
	log.Debug("OpenedStream called")
}

func (n *Notifee) ClosedStream(network.Network, network.Stream) {
	log.Debug("ClosedStream called")
}
