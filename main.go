package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"

	// pubsub "github.com/libp2p/go-libp2p-pubsub"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	// dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/muxer/mplex"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"

	// pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"

	// libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	// "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
)

// DiscoveryInterval is how often we re-publish our mDNS records.
const DiscoveryInterval = time.Hour

// DiscoveryServiceTag is used in our mDNS advertisements to discover other chat peers.
const DiscoveryServiceTag = "pubsub-chat-example"

const protocolName = "/libp2p/crcls/0.0.1"

var (
	roomFlag     = flag.String("room", "global", "name of topic to join")
	nameFlag     = flag.String("name", "", "Name to use in chat. will be generated if empty")
	relayFlag    = flag.Bool("relay", false, "Enable relay mode for this node.")
	useRelayFlag = flag.Bool("use-relay", false, "Use the relay node to bypass NAT/Firewalls")
)

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer %s\n", pi.ID.Pretty())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %s\n", pi.ID.Pretty(), err)
	}
}

func handleStream(stream network.Stream) {
	fmt.Println("Got a new stream!")

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

	// 'stream' will stay open until you close it (or the other side closes it).
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}

func startRelay(done chan bool) {
	h, err := libp2p.New()
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
		h.Close()
	}
}

func startClient(ctx context.Context, room string, done chan bool) {
	transports := libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(websocket.New),
	)

	muxers := libp2p.ChainOptions(
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
		libp2p.Muxer("/mplex/6.7.0", mplex.DefaultTransport),
	)

	security := libp2p.Security(tls.ID, tls.New)

	listenAddrs := libp2p.ListenAddrStrings(
		"/ip4/0.0.0.0/tcp/0",
		"/ip4/0.0.0.0/tcp/0/ws",
	)
	var dht *kaddht.IpfsDHT
	newDHT := func(h host.Host) (routing.PeerRouting, error) {
		var err error
		dht, err = kaddht.New(ctx, h)
		return dht, err
	}
	routing := libp2p.Routing(newDHT)

	// create a new libp2p Host that listens on a random TCP port
	h, err := libp2p.New(
		transports,
		muxers,
		security,
		listenAddrs,
		routing,
	)
	if err != nil {
		panic(err)
	}

	h.SetStreamHandler(protocol.ID(protocolName), handleStream)

	fmt.Printf("Host created. We are %s\n", h.ID())
	fmt.Println(h.Addrs())

	if *useRelayFlag {
		multi5, _ := multiaddr.NewMultiaddr("/ip4/172.20.10.5/tcp/58231")
		multi6, _ := multiaddr.NewMultiaddr("/ip4/172.20.10.5/udp/49941/quic")
		multi7, _ := multiaddr.NewMultiaddr("/ip4/172.20.10.5/udp/49941/quic-v1")
		multi8, _ := multiaddr.NewMultiaddr("/ip4/172.20.10.5/udp/57398/quic-v1/webtransport/certhash/uEiAZiRZx_LPE3l333zKQmznmZpo60o1WFLN97X9XpHVONA/certhash/uEiB7pzNWvVwEOK7eP6QAvUwt73eY4lGyyS91VzcA-6Ovxw")

		relayAddr := peer.AddrInfo{
			ID: "12D3KooWAGXSrH3DsCwFxmM2HXyR5xiKuTyaE46WEf8rPpCKtjMg",
			Addrs: []multiaddr.Multiaddr{
				multi5,
				multi6,
				multi7,
				multi8,
			},
		}

		if err := h.Connect(ctx, relayAddr); err != nil {
			fmt.Println("Failed to connect to the relay", err)
			done <- true
		}
	}

	select {
	case <-done:
		h.Close()
	}
}

func main() {
	// parse some flags to set our nickname and the room to join
	flag.Parse()

	ctx := context.Background()
	room := fmt.Sprintf("crcls-%s", *roomFlag)

	done := make(chan bool, 1)

	if *relayFlag {
		startRelay(done)
	} else {
		startClient(ctx, room, done)
	}

	// create a new PubSub service using the GossipSub router
	// ps, err := pubsub.NewGossipSub(ctx, h)
	// if err != nil {
	// 	panic(err)
	// }

	// // setup local mDNS discovery
	// // if err := setupLocalDiscovery(h); err != nil {
	// // 	panic(err)
	// // }

	// // use the nickname from the cli flag, or a default if blank
	// nick := *nickFlag
	// if len(nick) == 0 {
	// 	nick = defaultNick(h.ID())
	// }

	// join the chat room
	// _, err = JoinChatRoom(ctx, ps, h.ID(), *nameFlag, room)
	// if err != nil {
	// 	panic(err)
	// }

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	select {
	case <-stop:
		done <- true
		os.Exit(0)
	}
}

func discoverPeers(ctx context.Context, h host.Host, dht *kaddht.IpfsDHT, room string) {
	if err := dht.Bootstrap(ctx); err != nil {
		panic(err)
	}

	notifee := &discoveryNotifee{h}
	routingDiscovery := drouting.NewRoutingDiscovery(dht)
	routingDiscovery.Advertise(ctx, room)

	fmt.Println("Searching for peers...")
	peers, err := routingDiscovery.FindPeers(ctx, room)
	if err != nil {
		panic(err)
	}

	select {
	case peer := <-peers:

		if peer.ID != h.ID() {
			notifee.HandlePeerFound(peer)
		}
	}
}

// printErr is like fmt.Printf, but writes to stderr.
func printErr(m string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, m, args...)
}

// defaultNick generates a nickname based on the $USER environment variable and
// the last 8 chars of a peer ID.
func defaultNick(p peer.ID) string {
	return fmt.Sprintf("%s-%s", os.Getenv("USER"), shortID(p))
}

// shortID returns the last 8 chars of a base58-encoded peer id.
func shortID(p peer.ID) string {
	pretty := p.Pretty()
	return pretty[len(pretty)-8:]
}

// setupDiscovery creates an mDNS discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers on the same LAN and connect to them.
// func setupLocalDiscovery(h host.Host) error {
// 	// setup mDNS discovery to find local peers
// 	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
// 	return s.Start()
// }

// func initDHT(ctx context.Context, h host.Host) *dht.IpfsDHT {
// 	// Start a DHT, for use in peer discovery. We can't just make a new DHT
// 	// client because we want each peer to maintain its own local copy of the
// 	// DHT, so that the bootstrapping node of the DHT can go down without
// 	// inhibiting future peer discovery.
// 	kademliaDHT, err := dht.New(ctx, h)
// 	if err != nil {
// 		panic(err)
// 	}

// 	if err = kademliaDHT.Bootstrap(ctx); err != nil {
// 		panic(err)
// 	}
// 	var wg sync.WaitGroup
// 	for _, peerAddr := range dht.DefaultBootstrapPeers {
// 		peerinfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
// 		fmt.Println(*peerinfo)
// 		fmt.Println(err)
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			if err := h.Connect(ctx, *peerinfo); err != nil {
// 				fmt.Println("Bootstrap warning:", err)
// 			}
// 		}()
// 	}
// 	wg.Wait()

// 	return kademliaDHT
// }

// func discoverPeers(ctx context.Context, h host.Host, room string) {
// 	dht := initDHT(ctx, h)
// 	defer dht.Close()

// 	routingDiscovery := drouting.NewRoutingDiscovery(dht)
// 	fmt.Println(room)
// 	dutil.Advertise(ctx, routingDiscovery, room)

// 	// Look for others who have announced and attempt to connect to them
// 	anyConnected := false
// 	for !anyConnected {
// 		fmt.Println("Searching for peers...")
// 		peerChan, err := routingDiscovery.FindPeers(ctx, room)
// 		if err != nil {
// 			panic(err)
// 		}
// 		for peer := range peerChan {
// 			if peer.ID == h.ID() {
// 				continue // No self connection
// 			}
// 			err := h.Connect(ctx, peer)
// 			if err != nil {
// 				dht.RoutingTable().RemovePeer(peer.ID)
// 				fmt.Println("Failed connecting to ", peer.ID.Pretty(), ", error:", err)

// 			} else {
// 				fmt.Println("Connected to:", peer.ID.Pretty())
// 				anyConnected = true
// 			}
// 		}
// 	}
// 	fmt.Println("Peer discovery complete")
// }
