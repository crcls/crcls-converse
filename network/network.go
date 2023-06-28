package network

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/logger"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	// "github.com/libp2p/go-libp2p/core/peer"
	// "github.com/libp2p/go-libp2p/core/protocol"

	// "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
)

// DiscoveryServiceTag is used in our mDNS advertisements to discover other chat peers.
const DiscoveryServiceTag = "crcls-converse"

const ProtocolName = "/crcls"
const ProtocolVersion = "0.0.1"

var log = logger.GetLogger()

// Backoff discovery by increasing this value
var discInterval = time.Second

type ConnectionStatus struct {
	Error     error
	Connected bool
}

type Network struct {
	Peers     []peer.PeerRecord
	Port      int
	Connected bool
	Host      host.Host
	DHT       *kaddht.IpfsDHT
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

	log.Debugf("Host ID: %s", h.ID())

	h.Network().Notify(&Notifee{net})
	net.Host = h

	return nil
}

func (net *Network) initDHT(ctx context.Context) error {
	var dht *kaddht.IpfsDHT
	var err error

	if err != nil {
		return err
	}

	dht, err = kaddht.New(ctx, net.Host, kaddht.Mode(kaddht.ModeClient), kaddht.RoutingTableFilter(kaddht.PublicRoutingTableFilter))

	if err != nil {
		return err
	}

	peers := kaddht.GetDefaultBootstrapPeerAddrInfos()

	for _, peerAddr := range peers {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := net.Host.Connect(ctx, peerAddr); err != nil {
				log.Debugf("Error connecting to Bootstrap peer: %v", err)
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

func (net *Network) discoverPeers(ctx context.Context, statusChan chan ConnectionStatus) {
	routingDiscovery := drouting.NewRoutingDiscovery(net.DHT)

	// Let others know we are available to join.
	dutil.Advertise(ctx, routingDiscovery, ProtocolName)

	for {
		peers, err := routingDiscovery.FindPeers(ctx, ProtocolName)
		if err != nil {
			statusChan <- ConnectionStatus{
				Error:     err,
				Connected: false,
			}
		}

		for p := range peers {
			if p.ID != net.Host.ID() {
				pbytes, _ := p.MarshalJSON()
				log.Debugf("Peer: %v", string(pbytes))
				if err := net.Host.Connect(ctx, p); err == nil {
					log.Debugf("Connected to %s", p.ID)
					net.Peers = append(net.Peers, *peer.PeerRecordFromAddrInfo(p))

					statusChan <- ConnectionStatus{
						Error:     nil,
						Connected: true,
					}

					switch len(net.Peers) {
					case 10:
						discInterval = time.Minute
					case 100:
						discInterval = time.Minute * 30
					case 1000:
						discInterval = time.Hour
					}
				} else {
					log.Debugf("Failed to connect to %v", p.ID)
				}
			}
		}

		select {
		case <-time.After(discInterval):
		case <-ctx.Done():
			break

		}
	}
}

func (net *Network) Connect(ctx context.Context, acc *account.Account) chan ConnectionStatus {
	statusChan := make(chan ConnectionStatus)

	if err := net.startClient(ctx, acc.PK); err != nil {
		statusChan <- ConnectionStatus{
			Error:     err,
			Connected: false,
		}

		return statusChan
	}

	if err := net.initDHT(ctx); err != nil {
		statusChan <- ConnectionStatus{
			Error:     err,
			Connected: false,
		}

		return statusChan
	}

	go net.discoverPeers(ctx, statusChan)

	return statusChan
}

func New(port int) *Network {
	return &Network{
		Connected: false,
		Port:      port,
	}
}
