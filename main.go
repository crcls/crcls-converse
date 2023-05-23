package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	// "github.com/libp2p/go-libp2p/core/peerstore"
	// "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	// "github.com/multiformats/go-multiaddr"
)

// DiscoveryInterval is how often we re-publish our mDNS records.
const DiscoveryInterval = time.Hour

// DiscoveryServiceTag is used in our mDNS advertisements to discover other chat peers.
const DiscoveryServiceTag = "crcls-converse"

const ProtocolName = "/crcls/0.0.1"

var (
	roomFlag     = flag.String("room", "global", "name of topic to join")
	nameFlag     = flag.String("name", "", "Name to use in chat. will be generated if empty")
	relayFlag    = flag.Bool("relay", false, "Enable relay mode for this node.")
	useRelayFlag = flag.String("r", "", "AddrInfo Use the relay node to bypass NAT/Firewalls")
	portFlag     = flag.Int("p", 0, "PORT to connect on. 3123-3130")
	leaderFlag   = flag.Bool("leader", false, "Start this node in leader mode.")
	verboseFlag  = flag.Bool("v", false, "Verbose output")
)

var log = logging.Logger("crcls")

func startRelay() {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		panic(fmt.Sprintf("Failed to create the relay host: %v\n", err))
	}
	if _, err := relay.New(h); err != nil {
		panic(fmt.Sprintf("Failed to instantiate the relay: %v\n", err))
	}

	json, err := json.Marshal(host.InfoFromHost(h))
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal Relay AddrInfo: %v\n", err))
	}

	log.Infof("Relay AddrInfo: %s\n", json)
}

func startClient(ctx context.Context) (host.Host, error) {
	opts := libp2p.ChainOptions(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *portFlag)),
		libp2p.ProtocolVersion(ProtocolName),
	)
	// create a new libp2p Host that listens on a random TCP port
	h, err := libp2p.New(opts)
	if err != nil {
		panic(err)
	}

	if *useRelayFlag != "" {
		relayAddr := peer.AddrInfo{}
		if err := json.Unmarshal([]byte(*useRelayFlag), &relayAddr); err != nil {
			log.Errorf("Failed to unmarshal the relay node details: %v\n", err)
			return nil, err
		}

		if err := h.Connect(ctx, relayAddr); err != nil {
			log.Errorf("Failed to connnect to the relay: %v\n", err)
			return nil, err
		}
	}

	// h.SetStreamHandler(protocol.ID(protocolName), handleStream)

	log.Infof("host created. we are %s\n", h.ID())
	log.Infof("addrs: %v\n", h.Addrs())
	log.Infof("peers: %v\n", h.Network().Peers())

	return h, nil
}

func initDHT(ctx context.Context, h host.Host) *kaddht.IpfsDHT {
	dht, err := kaddht.New(ctx, h)
	if err != nil {
		panic(err)
	}

	if err = dht.Bootstrap(ctx); err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for _, peerAddr := range kaddht.DefaultBootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.Connect(ctx, *peerinfo); err != nil {
				log.Warn("Bootstrap warning:", err)
			}
		}()
	}
	wg.Wait()

	log.Info("Connected to the DHT")

	return dht
}

func isNewPeer(p peer.AddrInfo, h host.Host) bool {
	if p.ID == h.ID() {
		return false
	}

	peers := h.Network().Peers()

	for _, id := range peers {
		if id == p.ID {
			return false
		}
	}

	conn := h.Network().Connectedness(p.ID)

	return conn == network.CanConnect || conn == network.NotConnected
}

func discoverPeers(ctx context.Context, h host.Host, dht *kaddht.IpfsDHT, conChan chan bool) {
	routingDiscovery := drouting.NewRoutingDiscovery(dht)

	// Let others know we are available to join for one minute.
	dutil.Advertise(ctx, routingDiscovery, ProtocolName, discovery.TTL(time.Minute))

	peers, err := routingDiscovery.FindPeers(ctx, ProtocolName)
	if err != nil {
		panic(err)
	}

	connected := false
	for !connected {
		select {
		case peer := <-peers:
			if isNewPeer(peer, h) && len(peer.ID) != 0 && len(peer.Addrs) != 0 {
				if err := h.Connect(ctx, peer); err != nil {
					log.Error(err)
					dht.RoutingTable().RemovePeer(peer.ID)
					h.Peerstore().RemovePeer(peer.ID)
				} else {
					log.Infof("Connected to %s\n", peer.ID)
					connected = true
					conChan <- true
				}
			}
		case <-ctx.Done():
			return
		}
	}

	log.Info("Connected to CRCLS")
}

func main() {
	flag.Parse()

	var lvl logging.LogLevel

	var err error
	if *verboseFlag {
		lvl, err = logging.LevelFromString("debug")
	} else {
		lvl, err = logging.LevelFromString("error")
	}

	if err != nil {
		panic(err)
	}
	logging.SetAllLoggers(lvl)

	ctx, cancel := context.WithCancel(context.Background())
	room := fmt.Sprintf("crcls-%s", *roomFlag)

	conChan := make(chan bool, 1)

	if *relayFlag {
		startRelay()
	} else {
		h, _ := startClient(ctx)
		dht := initDHT(ctx, h)

		ps, err := pubsub.NewGossipSub(ctx, h)
		if err != nil {
			panic(err)
		}

		if *leaderFlag {
			// initialize the chat rooms
			// TODO: get a persisted list of rooms from somewhere
			global, err := ps.Join("crcls-global")
			if err != nil {
				panic(err)
			}
			if _, err := global.Subscribe(); err != nil {
				panic(err)
			}
		}

		go discoverPeers(ctx, h, dht, conChan)

		// setup local mDNS discovery
		// if err := setupLocalDiscovery(h); err != nil {
		// 	panic(err)
		// }
		select {
		case <-conChan:
			if !*leaderFlag {
				// create a new PubSub service using the GossipSub router
				_, err = JoinChatRoom(ctx, ps, h.ID(), *nameFlag, room, log)
				if err != nil {
					panic(err)
				}
			}
		case <-ctx.Done():
			h.Peerstore().ClearAddrs(h.ID())
			h.Peerstore().RemovePeer(h.ID())
			h.Close()
		}
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	select {
	case <-stop:
		cancel()
		os.Exit(0)

	}
}

// setupDiscovery creates an mDNS discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers on the same LAN and connect to them.
// func setupLocalDiscovery(h host.Host) error {
// 	// setup mDNS discovery to find local peers
// 	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
// 	return s.Start()
// }
