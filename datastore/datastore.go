package datastore

import (
	"context"
	"crcls-converse/config"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crypto/ecdsa"
	"fmt"
	"strings"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	ipfsDs "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	crdt "github.com/ipfs/go-ds-crdt"
	logging "github.com/ipfs/go-log/v2"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
)

type EntryEvent struct {
	Key   ipfsDs.Key
	Value []byte
}

func (ee *EntryEvent) Sender() string {
	base := strings.Split(ee.Key.BaseNamespace(), ":")
	return base[1]
}

type Datastore struct {
	PK          *ecdsa.PrivateKey
	EventStream chan EntryEvent
	crdt        *crdt.Datastore
	ds          *ValidatorDatastore
	log         *logging.ZapEventLogger
	pubsub      *PubSubBroadcaster
	subs        *Subscriptions
}

var internalDs *Datastore

// TODO: need a home dot directory for these things
const STORE_DIR = "ds"
const CRCLS_NS = "/crcls/datastore/1.0.0"

func NewDatastore(ctx context.Context, conf *config.Config, h *host.Host, ps *pubsub.PubSub, dht *kaddht.IpfsDHT) *Datastore {
	log := logger.GetLogger()

	subs := NewSubscriptions()

	vds, err := NewValidatorDatastore(conf)
	if err != nil {
		inout.EmitError(err)
		return nil
	}
	// Create the Broadcaster
	bcast, err := NewPubSubBroadcaster(ctx, ps, CRCLS_NS)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	// Create the DAG
	dag, err := ipfslite.New(ctx, vds, nil, *h, dht, nil)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	copts := crdt.DefaultOptions()
	copts.Logger = log
	copts.RebroadcastInterval = time.Second * 33
	copts.DAGSyncerTimeout = time.Second * 33
	copts.PutHook = subs.Propagate

	c, err := crdt.New(vds, ipfsDs.NewKey(CRCLS_NS), dag, bcast, copts)
	if err != nil {
		log.Fatal(err)
	}

	ds := &Datastore{
		crdt:   c,
		ds:     vds,
		log:    log,
		pubsub: bcast,
		subs:   subs,
	}

	internalDs = ds

	return ds
}

func (ds *Datastore) Authenticate(pk *ecdsa.PrivateKey) {
	ds.PK = pk
	ds.pubsub.Authenticate(pk)
}

func (ds *Datastore) Get(ctx context.Context, key ipfsDs.Key) ([]byte, error) {
	return ds.crdt.Get(ctx, key)
}

func (ds *Datastore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	loc, err := ds.crdt.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	return loc, nil
}

func (ds *Datastore) Put(ctx context.Context, key ipfsDs.Key, value []byte) error {
	if ds.PK == nil {
		return fmt.Errorf("Not authenticated")
	}

	return ds.crdt.Put(ctx, key, value)
}

func (ds *Datastore) Close() error {
	if err := ds.crdt.Close(); err != nil {
		return err
	}

	return nil
}

func (ds *Datastore) Stats() crdt.Stats {
	return ds.crdt.InternalStats()
}

func (ds *Datastore) Subscribe(prefix ipfsDs.Key) chan *inout.Message {
	return ds.subs.Subscribe(prefix)
}

func (ds *Datastore) Unsubscribe(prefix ipfsDs.Key) error {
	return ds.subs.Unsubscribe(prefix)
}
