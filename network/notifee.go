package network

import (
	logging "github.com/ipfs/go-log/v2"
	net "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type Notifee struct {
	net *Network
	log *logging.ZapEventLogger
}

func (n *Notifee) Listen(net.Network, ma.Multiaddr) {
	n.log.Debug("Listen called")
}

func (n *Notifee) ListenClose(net.Network, ma.Multiaddr) {
	n.log.Debug("ListenClose called")
}

func (n *Notifee) Connected(netw net.Network, con net.Conn) {
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

func (n *Notifee) Disconnected(netw net.Network, con net.Conn) {
	peerId := con.RemotePeer()
	pk, err := PeerIdToPublicKey(peerId)
	if err != nil {
		return
	}

	for i, p := range n.net.Peers {
		pkp, err := PeerIdToPublicKey(p)
		if err != nil {
			n.log.Fatal(err)
		}
		if pk.Equal(pkp) {
			n.log.Debugf("Peer %s has disconnected", peerId)

			if len(n.net.Peers) == 1 {
				n.net.Peers = make([]peer.ID, 0)
			} else if i == 0 {
				n.net.Peers = n.net.Peers[1:]
			} else if i == len(n.net.Peers)-1 {
				n.net.Peers = n.net.Peers[:len(n.net.Peers)-2]
			} else {
				start := n.net.Peers[:i-1]
				end := n.net.Peers[i:]
				n.net.Peers = append(start, end...)
			}

			n.net.StatusChan <- ConnectionStatus{
				Connected: false,
				Peer:      peerId,
			}
		}
	}
}

func (n *Notifee) OpenedStream(net.Network, net.Stream) {
	n.log.Debug("OpenedStream called")
}

func (n *Notifee) ClosedStream(net.Network, net.Stream) {
	n.log.Debug("ClosedStream called")
}
