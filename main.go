package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log/v2"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/discovery"

	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	// "github.com/libp2p/go-libp2p/core/discovery"
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
	isLeaderFlag = flag.Bool("leader", false, "Start this node in leader mode.")
	leaderFlag   = flag.String("l", "", "Connect to a known leader")
	verboseFlag  = flag.Bool("v", false, "Verbose output")
)

var log = logging.Logger("crcls")

func startRelay() {
	identity := getIdentity("./crcls_keyfile.pem")
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"), libp2p.Identity(identity), libp2p.EnableRelayService())
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

	fmt.Println(string(json))
}

func startClient(ctx context.Context) (host.Host, error) {
	identity := getIdentity("./crcls_keyfile.pem")
	listenOpt := libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *portFlag))

	opts := libp2p.ChainOptions(
		libp2p.ProtocolVersion(ProtocolName),
		libp2p.Identity(identity),
	)

	var h host.Host
	var err error

	if *useRelayFlag != "" {
		relayAddr := peer.AddrInfo{}
		if err = json.Unmarshal([]byte(*useRelayFlag), &relayAddr); err != nil {
			log.Errorf("Failed to unmarshal the relay node details: %v", err)
			return nil, err
		}

		hostId, err := peer.IDFromPrivateKey(identity)
		if err != nil {
			panic(err)
		}

		log.Debug("Host ID: ", hostId.String())

		relayHostAddr := filepath.Join(relayAddr.Addrs[len(relayAddr.Addrs)-1].String(), "p2p", relayAddr.ID.String(), "p2p-circuit", "p2p", hostId.String())
		log.Debug("RelayHostMultiAddr: ", relayHostAddr)

		listenOpt = libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *portFlag), relayHostAddr)
		// create a new libp2p Host that listens on a random TCP port
		h, err = libp2p.New(opts, listenOpt)
		if err != nil {
			panic(err)
		}

		// TODO: handle reservation expiration
		reservation, err := client.Reserve(ctx, h, relayAddr)
		if err != nil {
			log.Errorf("Host failed to receive a relay reservation. %v", err)
		} else {
			log.Debugf("Relay Reservation: %+v", reservation)
		}

		if err := h.Connect(ctx, relayAddr); err != nil {
			log.Errorf("Failed to connnect to the relay: %v", err)
			return nil, err
		}
	} else {
		h, err = libp2p.New(opts, listenOpt)
		if err != nil {
			panic(err)
		}
	}

	if *leaderFlag != "" {
		leaderAddr := peer.AddrInfo{}

		if err := json.Unmarshal([]byte(*leaderFlag), &leaderAddr); err != nil {
			log.Errorf("Failed to unmarshal the leader node details: %v", err)
			return nil, err
		}

		if err := h.Connect(ctx, leaderAddr); err != nil {
			log.Errorf("Failed to connnect to the leader: %v", err)
			return nil, err
		}
	}

	// h.SetStreamHandler(protocol.ID(protocolName), handleStream)

	log.Infof("host created. we are %s", h.ID())
	log.Infof("addrs: %v", h.Addrs())
	log.Infof("peers: %v", h.Network().Peers())

	return h, nil
}

func initDHT(ctx context.Context, h host.Host) *kaddht.IpfsDHT {
	dht, err := kaddht.New(ctx, h, kaddht.ProtocolPrefix("/crcls"))
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for _, peerAddr := range kaddht.GetDefaultBootstrapPeerAddrInfos() {
		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()
			if err := h.Connect(ctx, p); err != nil {
				log.Warn("Bootstrap warning:", err)
			} else {
				log.Debugf("Connected to Bootstrap peer: %s", p.ID)
			}
		}(peerAddr)
	}
	wg.Wait()

	if err = dht.Bootstrap(ctx); err != nil {
		panic(err)
	}

	log.Debug("Connected to the DHT")

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

	// Let others know we are available to join for ten minutes.
	dutil.Advertise(ctx, routingDiscovery, ProtocolName, discovery.TTL(time.Minute*10))

	log.Info("Discovering peers...")

	connected := false
	for !connected {
		peers, err := routingDiscovery.FindPeers(ctx, ProtocolName)
		if err != nil {
			panic(err)
		}

		for peer := range peers {
			if len(peer.ID) != 0 && len(peer.Addrs) != 0 {
				if isNewPeer(peer, h) {
					log.Debug(peer)
					if err := h.Connect(ctx, peer); err != nil {
						log.Warn(err)
					} else {
						log.Infof("Connected to %s", peer.ID)
						connected = true
						conChan <- true
					}
				} else if peer.ID != h.ID() {
					log.Debug(peer)
					h.Peerstore().RemovePeer(peer.ID)
					dht.RoutingTable().RemovePeer(peer.ID)
				}
			}
		}
	}

	log.Info("Connected to CRCLS")
}

func getIdentity(keyFile string) crypto.PrivKey {
	var priv crypto.PrivKey
	var err error
	if _, err = os.Stat(keyFile); os.IsNotExist(err) {
		// Key file does not exist, create a new one
		priv, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			panic(err)
		}

		// save the private key to file
		keyBytes, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(keyFile, keyBytes, 0600)
		if err != nil {
			panic(err)
		}
	} else {
		// Load key from file
		keyBytes, err := ioutil.ReadFile(keyFile)
		if err != nil {
			panic(err)
		}
		priv, err = crypto.UnmarshalPrivateKey(keyBytes)
		if err != nil {
			panic(err)
		}
	}

	return priv
}

func main() {
	flag.Parse()

	if *verboseFlag {
		logging.SetLogLevel("crcls", "debug")
	} else {
		logging.SetLogLevel("crcls", "info")
	}

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

		if *isLeaderFlag {
			// initialize the chat rooms
			// TODO: get a persisted list of rooms from somewhere
			global, err := ps.Join("crcls-global")
			if err != nil {
				panic(err)
			}

			if _, err := global.Subscribe(); err != nil {
				panic(err)
			}

			log.Debug("Connected to: ", ps.GetTopics())

			json, err := json.Marshal(host.InfoFromHost(h))
			if err != nil {
				panic(fmt.Sprintf("Failed to marshal Leader AddrInfo: %v\n", err))
			}

			fmt.Printf("Leader: %s\n", string(json))
		}

		go discoverPeers(ctx, h, dht, conChan)

		// setup local mDNS discovery
		// if err := setupLocalDiscovery(h); err != nil {
		// 	panic(err)
		// }
		select {
		case <-conChan:
			if !*isLeaderFlag {
				log.Debug("Joining the chat room...")
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
