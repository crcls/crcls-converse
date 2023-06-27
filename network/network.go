package network

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/logger"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	// "github.com/libp2p/go-libp2p/core/peer"
	// "github.com/libp2p/go-libp2p/core/protocol"

	// "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

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
	Peers     int
	Connected bool
	Host      host.Host
}

func startRelay(h host.Host) {
	if _, err := relay.New(h); err != nil {
		panic(fmt.Sprintf("Failed to instantiate the relay: %v\n", err))
	}

	json, err := json.Marshal(host.InfoFromHost(h))
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal Relay AddrInfo: %v\n", err))
	}

	log.Infof("Relay: %s", string(json))
}

func startClient(ctx context.Context, port int, identity crypto.PrivKey) (host.Host, error) {
	opts := libp2p.ChainOptions(
		libp2p.ProtocolVersion(ProtocolVersion),
		libp2p.Identity(identity),
		libp2p.Security(tls.ID, tls.New),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)),
	)

	h, err := libp2p.New(opts)
	if err != nil {
		return nil, err
	}

	log.Debugf("Host ID: %s", h.ID())

	return h, nil
}

func initDHT(ctx context.Context, h host.Host) (*kaddht.IpfsDHT, error) {
	var dht *kaddht.IpfsDHT
	var err error

	if err != nil {
		return nil, err
	}

	dht, err = kaddht.New(ctx, h, kaddht.Mode(kaddht.ModeClient), kaddht.RoutingTableFilter(kaddht.PublicRoutingTableFilter))

	if err != nil {
		return dht, err
	}

	peers := kaddht.GetDefaultBootstrapPeerAddrInfos()

	for _, peerAddr := range peers {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.Connect(ctx, peerAddr); err != nil {
				log.Debugf("Error connecting to Bootstrap peer: %v", err)
			}
		}()
		wg.Wait()
	}

	if err = dht.Bootstrap(ctx); err != nil {
		return dht, err
	}

	return dht, nil
}

func discoverPeers(ctx context.Context, h host.Host, dht *kaddht.IpfsDHT, statusChan chan ConnectionStatus) {
	routingDiscovery := drouting.NewRoutingDiscovery(dht)

	// Let others know we are available to join.
	dutil.Advertise(ctx, routingDiscovery, ProtocolName)

	for {
		peers, err := routingDiscovery.FindPeers(ctx, ProtocolName)
		if err != nil {
			statusChan <- ConnectionStatus{
				Error:     err,
				Peers:     h.Peerstore().Peers().Len(),
				Connected: false,
				Host:      h,
			}
		}

		for p := range peers {
			if p.ID != h.ID() {
				pbytes, _ := p.MarshalJSON()
				log.Debugf("Peer: %v", string(pbytes))
				if err := h.Connect(ctx, p); err == nil {
					log.Debugf("Connected to %s", p.ID)
					peerCount := h.Peerstore().Peers().Len()

					statusChan <- ConnectionStatus{
						Error:     nil,
						Peers:     peerCount,
						Connected: true,
						Host:      h,
					}

					if peerCount > 6 {
						discInterval = time.Second * 30
					} else if peerCount > 12 {
						discInterval = time.Minute
					} else if peerCount > 18 {
						discInterval = time.Minute * 30
					} else if peerCount > 30 {
						discInterval = time.Hour
					}
				}
			}
		}

		time.Sleep(discInterval)
	}
}

func Connect(ctx context.Context, port int, account *account.Account) chan ConnectionStatus {
	statusChan := make(chan ConnectionStatus)

	h, err := startClient(ctx, port, account.PK)

	if err != nil {
		statusChan <- ConnectionStatus{
			Error:     err,
			Peers:     0,
			Connected: false,
			Host:      h,
		}

		return statusChan
	}

	dht, err := initDHT(ctx, h)

	if err != nil {
		statusChan <- ConnectionStatus{
			Error:     err,
			Peers:     0,
			Connected: false,
			Host:      h,
		}

		return statusChan
	}

	go discoverPeers(ctx, h, dht, statusChan)

	return statusChan
}
