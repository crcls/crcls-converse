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

	// "path/filepath"
	"sync"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log/v2"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/discovery"

	// "github.com/libp2p/go-libp2p/core/protocol"

	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"

	// "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	// "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	// "github.com/libp2p/go-libp2p/core/peerstore"
	// "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	// "github.com/multiformats/go-multiaddr"
	// flatfs "github.com/ipfs/go-ds-flatfs"
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
	verboseFlag  = flag.Bool("v", false, "Verbose output")
)

var log = logging.Logger("crcls")

func startRelay(h host.Host) {
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

	opts := libp2p.ChainOptions(
		libp2p.ProtocolVersion(ProtocolName),
		libp2p.Identity(identity),
		libp2p.Security(tls.ID, tls.New),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *portFlag)),
	)

	h, err := libp2p.New(opts)
	if err != nil {
		panic(err)
	}

	log.Infof("host created. we are %s", h.ID())
	log.Infof("addrs: %v", h.Addrs())
	log.Infof("peers: %v", h.Network().Peers())

	return h, nil
}

func initDHT(ctx context.Context, h host.Host) *kaddht.IpfsDHT {
	var dht *kaddht.IpfsDHT
	var err error
	// ds, err := flatfs.createoropen("./crcls-ds", flatfs.prefix(1), true)

	if err != nil {
		panic(err)
	}

	if *relayFlag {
		dht, err = kaddht.New(ctx, h, kaddht.Mode(kaddht.ModeServer))
	} else {
		dht, err = kaddht.New(ctx, h, kaddht.Mode(kaddht.ModeClient), kaddht.RoutingTableFilter(kaddht.PublicRoutingTableFilter))
	}

	if err != nil {
		panic(err)
	}

	peers := kaddht.GetDefaultBootstrapPeerAddrInfos()

	if *useRelayFlag != "" {
		relayAddr := peer.AddrInfo{}
		if err := json.Unmarshal([]byte(*useRelayFlag), &relayAddr); err != nil {
			panic(err)
		}

		fmt.Println(relayAddr.Addrs[1])

		peers = append(peers, relayAddr)
	}

	for _, peerAddr := range peers {
		var wg sync.WaitGroup
		fmt.Printf("%v\n", peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.Connect(ctx, peerAddr); err != nil {
				log.Warn("Bootstrap warning:", err)
			} else {
				log.Debugf("Connected to Bootstrap peer: %s", peerAddr.ID)
			}
		}()
		wg.Wait()
	}

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
	dutil.Advertise(ctx, routingDiscovery, "crcls", discovery.TTL(time.Minute*10))

	log.Info("Discovering peers on crcls...")

	connected := false
	for !connected {
		peers, err := routingDiscovery.FindPeers(ctx, "crcls")
		if err != nil {
			panic(err)
		}

		for p := range peers {
			if len(p.ID) != 0 && len(p.Addrs) != 0 {
				if isNewPeer(p, h) {
					log.Debug(p)

					if err := h.Connect(ctx, p); err != nil {
						log.Debug(err)
						h.Peerstore().RemovePeer(p.ID)
						dht.RoutingTable().RemovePeer(p.ID)
					} else {
						log.Infof("Connected to %s", p.ID)
						if !*relayFlag {
							connected = true
							conChan <- true
						}
					}
				} else if p.ID != h.ID() {
					log.Debug(p)
					h.Peerstore().RemovePeer(p.ID)
					dht.RoutingTable().RemovePeer(p.ID)
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

	h, _ := startClient(ctx)
	dht := initDHT(ctx, h)

	// fmt.Println(dht.RoutingTable().GetPeerInfos())

	go discoverPeers(ctx, h, dht, conChan)

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}

	if *relayFlag {
		startRelay(h)

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
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	select {
	case <-conChan:
		log.Debug("Joining the chat room...")
		// create a new PubSub service using the GossipSub router
		errors := make(chan error, 1)
		JoinChatRoom(ctx, ps, h.ID(), *nameFlag, room, log, errors)

		select {
		case err := <-errors:
			fmt.Println(err)
			cancel()
			os.Exit(1)
		case <-stop:
			cancel()
			os.Exit(0)
		}
	case <-ctx.Done():
		h.Peerstore().ClearAddrs(h.ID())
		h.Peerstore().RemovePeer(h.ID())
		h.Close()
	case <-stop:
		cancel()
		os.Exit(0)
	}
}

// setup local mDNS discovery
// if err := setupLocalDiscovery(h); err != nil {
// 	panic(err)
// }
// setupDiscovery creates an mDNS discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers on the same LAN and connect to them.
// func setupLocalDiscovery(h host.Host) error {
// 	// setup mDNS discovery to find local peers
// 	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
// 	return s.Start()
// }
