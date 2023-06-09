package network

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/config"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/libp2p/go-libp2p/core/host"

	logging "github.com/ipfs/go-log/v2"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
)

// DiscoveryServiceTag is used in our mDNS advertisements to discover other chat peers.
const DiscoveryServiceTag = "crcls-converse"

const ProtocolName = "/crcls"
const ProtocolVersion = "0.0.1"

// Backoff discovery by increasing this value
var discInterval = time.Second

type ConnectionStatus struct {
	Error     error            `json:"error"`
	Connected bool             `json:"connected"`
	Peer      *peer.PeerRecord `json:"peer"`
}

type Network struct {
	Peers      []*peer.PeerRecord
	Port       int
	PubSub     *pubsub.PubSub
	Connected  bool
	Host       *host.Host
	DHT        *kaddht.IpfsDHT
	StatusChan chan ConnectionStatus
	log        *logging.ZapEventLogger
}

func (net *Network) startClient(ctx context.Context, identity crypto.PrivKey) error {
	opts := libp2p.ChainOptions(
		libp2p.ProtocolVersion(ProtocolVersion),
		libp2p.Identity(identity),
		libp2p.Security(tls.ID, tls.New),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", net.Port)),
	)

	h, err := libp2p.New(opts)
	if err != nil {
		return err
	}

	h.Network().Notify(&Notifee{net: net, log: net.log})
	net.Host = &h

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	net.PubSub = ps

	return nil
}

func (net *Network) initDHT(ctx context.Context) error {
	var dht *kaddht.IpfsDHT
	var err error

	if err != nil {
		return err
	}

	dht, err = kaddht.New(ctx, *net.Host, kaddht.Mode(kaddht.ModeClient), kaddht.RoutingTableFilter(kaddht.PublicRoutingTableFilter))

	if err != nil {
		return err
	}

	peers := kaddht.GetDefaultBootstrapPeerAddrInfos()

	for _, peerAddr := range peers {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := (*net.Host).Connect(ctx, peerAddr); err != nil {
				net.log.Debugf("Error connecting to Bootstrap peer: %v", err)
			}
		}()
		wg.Wait()
	}

	if err = dht.Bootstrap(ctx); err != nil {
		return err
	}

	net.DHT = dht

	return nil
}

func (net *Network) isNewPeer(peer peer.AddrInfo) bool {
	if (*net.Host).ID() == peer.ID {
		return false
	}

	for _, p := range net.Peers {
		if p.PeerID == peer.ID {
			return false
		}
	}

	return true
}

func (net *Network) discoverPeers(ctx context.Context) {
	routingDiscovery := drouting.NewRoutingDiscovery(net.DHT)

	// Let others know we are available to join.
	dutil.Advertise(ctx, routingDiscovery, ProtocolName)

	for {
		peers, err := routingDiscovery.FindPeers(ctx, ProtocolName)
		if err != nil {
			net.StatusChan <- ConnectionStatus{
				Error:     err,
				Connected: false,
			}
		}

		for p := range peers {
			if net.isNewPeer(p) {
				if err := (*net.Host).Connect(ctx, p); err == nil {
					net.log.Debugf("Connected to %s", p.ID)
					pr := peer.PeerRecordFromAddrInfo(p)
					net.Peers = append(net.Peers, pr)

					net.StatusChan <- ConnectionStatus{
						Error:     nil,
						Connected: true,
						Peer:      pr,
					}

					net.log.Debugf("Peer count %d", len(net.Peers))
					switch len(net.Peers) {
					case 10:
						discInterval = time.Minute
					case 100:
						discInterval = time.Minute * 30
					case 1000:
						discInterval = time.Hour
					}
				}
			}
		}

		select {
		case <-time.After(discInterval):
		case <-ctx.Done():
			net.log.Debug("Discovery ended")
			break

		}
	}
}

func (net *Network) Connect(ctx context.Context, acc *account.Account) {
	if err := net.startClient(ctx, acc.HostPk); err != nil {
		net.StatusChan <- ConnectionStatus{
			Error:     err,
			Connected: false,
		}
	}

	if err := net.initDHT(ctx); err != nil {
		net.StatusChan <- ConnectionStatus{
			Error:     err,
			Connected: false,
		}
	}

	go net.discoverPeers(ctx)
}

func New(conf *config.Config) *Network {
	log := logger.GetLogger()
	return &Network{
		Connected:  false,
		Port:       conf.Port,
		StatusChan: make(chan ConnectionStatus),
		Peers:      make([]*peer.PeerRecord, 0),
		log:        log,
	}
}
