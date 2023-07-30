package network

import (
	"context"
	"crcls-converse/config"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	// ethcrypto "github.com/ethereum/go-ethereum/crypto"

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
	Error     error   `json:"error"`
	Connected bool    `json:"connected"`
	Peer      peer.ID `json:"peer"`
}

type Network struct {
	PrivKey    crypto.PrivKey
	PubKey     *ecdsa.PublicKey
	Peers      []peer.ID
	Dead       []peer.ID
	Port       int
	PubSub     *pubsub.PubSub
	Connected  bool
	Host       *host.Host
	DHT        *kaddht.IpfsDHT
	StatusChan chan ConnectionStatus
	log        *logging.ZapEventLogger
}

func (net *Network) startClient(ctx context.Context) error {
	opts := libp2p.ChainOptions(
		libp2p.ProtocolVersion(ProtocolVersion),
		libp2p.Identity(net.PrivKey),
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
	pk, _ := PeerIdToPublicKey(peer.ID)
	if pk == nil {
		return false
	}

	if net.PubKey.Equal(pk) {
		return false
	}

	for _, p := range net.Peers {
		pkp, err := PeerIdToPublicKey(p)
		if err != nil {
			net.log.Debug(err)
			continue
		}
		if pk.Equal(pkp) {
			return false
		}
	}

	for _, p := range net.Dead {
		pkp, err := PeerIdToPublicKey(p)
		if err != nil {
			continue
		}
		if pk.Equal(pkp) {
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
					net.Peers = append(net.Peers, p.ID)

					net.StatusChan <- ConnectionStatus{
						Error:     nil,
						Connected: true,
						Peer:      p.ID,
					}

					net.log.Debugf("Peer count %d", len(net.Peers))
					l := len(net.Peers)
					if l < 10 {
						discInterval = time.Second
					} else if l < 100 {
						discInterval = time.Minute
					} else if l < 1000 {
						discInterval = time.Minute * 30
					} else {
						discInterval = time.Hour
					}
				} else {
					net.Dead = append(net.Dead, p.ID)
				}
			} else {
				net.Dead = append(net.Dead, p.ID)
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

func (net *Network) Connect(ctx context.Context) {
	if err := net.startClient(ctx); err != nil {
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

func New(ctx context.Context, conf *config.Config, key *ecdsa.PrivateKey) (*Network, error) {
	log := logger.GetLogger()

	privKey, err := crypto.UnmarshalSecp256k1PrivateKey(key.D.Bytes())
	if err != nil {
		return nil, err
	}

	net := &Network{
		PrivKey:    privKey,
		PubKey:     &key.PublicKey,
		Connected:  false,
		Port:       conf.Port,
		StatusChan: make(chan ConnectionStatus),
		Peers:      make([]peer.ID, 0),
		log:        log,
	}

	net.Connect(ctx)

	return net, nil
}
