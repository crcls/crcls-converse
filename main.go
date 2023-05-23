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
	useRelayFlag = flag.String("use-relay", "", "Use the relay node to bypass NAT/Firewalls")
	portFlag     = flag.Int("port", 0, "PORT to connect on. 3123-3130")
	leaderFlag   = flag.Bool("leader", false, "Start this node in leader mode.")
)

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.h.Connect(context.Background(), pi)
}

func startRelay(done chan bool) {
	h, err := libp2p.New(libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *portFlag)))
	if err != nil {
		fmt.Printf("Failed to create relay1: %v\n", err)
		done <- true
	}

	r, err := relay.New(h)
	if err != nil {
		fmt.Printf("Failed to instantiate the relay: %v\n", err)
		done <- true
	}

	json, err := json.Marshal(host.InfoFromHost(h))
	if err != nil {
		fmt.Printf("Failed to marshal Relay AddrInfo: %v\n", err)
	}

	fmt.Printf("Relay AddrInfo: %s\n", json)

	select {
	case <-done:
		r.Close()
	}
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
			fmt.Printf("Failed to unmarshal the relay node details: %v\n", err)
			return nil, err
		}

		if err := h.Connect(ctx, relayAddr); err != nil {
			fmt.Printf("Failed to connnect to the relay: %v\n", err)
			return nil, err
		}
	}

	// h.SetStreamHandler(protocol.ID(protocolName), handleStream)

	fmt.Printf("Host created. We are %s\n", h.ID())
	fmt.Printf("Addrs: %v\n", h.Addrs())
	fmt.Printf("Peers: %v\n", h.Network().Peers())
	// fmt.Printf("Connections: %v\n", h.Network().Conns())

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
				fmt.Println("Bootstrap warning:", err)
			}
		}()
	}
	wg.Wait()

	fmt.Println("Connected to the DHT")

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

func discoverPeers(ctx context.Context, h host.Host, dht *kaddht.IpfsDHT) {
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
					// fmt.Println(err)
					dht.RoutingTable().RemovePeer(peer.ID)
					h.Peerstore().RemovePeer(peer.ID)
				} else {
					fmt.Printf("Connected to %s\n", peer.ID)
					connected = true
				}
			}
		case <-ctx.Done():
			return
		}
	}

	fmt.Println("Connected to CRCLS")
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	room := fmt.Sprintf("crcls-%s", *roomFlag)

	done := make(chan bool, 1)

	if *relayFlag {
		startRelay(done)
	} else {
		h, _ := startClient(ctx)
		dht := initDHT(ctx, h)

		go discoverPeers(ctx, h, dht)

		// create a new PubSub service using the GossipSub router
		ps, err := pubsub.NewGossipSub(ctx, h)
		if err != nil {
			panic(err)
		}

		// setup local mDNS discovery
		// if err := setupLocalDiscovery(h); err != nil {
		// 	panic(err)
		// }

		// join the chat room if you are not a leader
		if !*leaderFlag {
			_, err = JoinChatRoom(ctx, ps, h.ID(), *nameFlag, room)
			if err != nil {
				panic(err)
			}
		}

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT)

		select {
		case <-stop:
			cancel()
			h.Peerstore().ClearAddrs(h.ID())
			h.Peerstore().RemovePeer(h.ID())
			h.Close()

			done <- true
			os.Exit(0)
		}
	}
}

// setupDiscovery creates an mDNS discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers on the same LAN and connect to them.
// func setupLocalDiscovery(h host.Host) error {
// 	// setup mDNS discovery to find local peers
// 	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
// 	return s.Start()
// }
