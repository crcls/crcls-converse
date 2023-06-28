package network

import (
	"github.com/libp2p/go-libp2p/core/network"
	// "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type Notifee struct {
	net *Network
}

func (n *Notifee) Listen(network.Network, ma.Multiaddr) {
	log.Debug("Listen called")
}

func (n *Notifee) ListenClose(network.Network, ma.Multiaddr) {
	log.Debug("ListenClose called")
}

func (n *Notifee) Connected(net network.Network, con network.Conn) {
	// peerRecord := net.Peerstore().PeerInfo(con.RemotePeer())

	// adding := true
	// for _, peer := range n.net.Peers {
	// 	if peer.PeerID == peerRecord.ID {
	// 		adding = false
	// 	}
	// }

	// if adding {
	// 	n.net.Peers = append(n.net.Peers, *peer.PeerRecordFromAddrInfo(peerRecord))
	// 	log.Debugf("Connected to %v", peerRecord.ID)
	// }
}

func (n *Notifee) Disconnected(net network.Network, con network.Conn) {
	peerRecord := net.Peerstore().PeerInfo(con.RemotePeer())

	for i, peer := range n.net.Peers {
		if peer.PeerID == peerRecord.ID {
			log.Debugf("Peer %s has disconnected", peerRecord.ID)

			if i == len(n.net.Peers)-1 {
				n.net.Peers = n.net.Peers[:len(n.net.Peers)-2]
			} else if i == 0 {
				n.net.Peers = n.net.Peers[1:]
			} else {
				start := n.net.Peers[:i-1]
				end := n.net.Peers[i:]
				n.net.Peers = append(start, end...)
			}
		}
	}
}

func (n *Notifee) OpenedStream(network.Network, network.Stream) {
	log.Debug("OpenedStream called")
}

func (n *Notifee) ClosedStream(network.Network, network.Stream) {
	log.Debug("ClosedStream called")
}
