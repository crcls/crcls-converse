package datastore

import (
	"context"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"strings"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	ipfsDs "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"

	// "github.com/ipfs/go-datastore/query"
	badger "github.com/ipfs/go-ds-badger3"
	crdt "github.com/ipfs/go-ds-crdt"
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
	Badger      *badger.Datastore
	EventStream chan EntryEvent
	crdt        *crdt.Datastore
}

var log = logger.GetLogger()
var internalDs *Datastore

// TODO: need a home dot directory for these things
const STORE_DIR = "./crcls-ds"
const CRCLS_NS = "/crcls/datastore/1.0.0"

func NewDatastore(ctx context.Context, h host.Host, ps *pubsub.PubSub, dht *kaddht.IpfsDHT) *Datastore {
	// opts := badger.Options{
	// 	GcDiscardRatio: 0,
	// 	GcInterval:     0,
	// }
	d, err := badger.NewDatastore(STORE_DIR, &badger.DefaultOptions)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	bcast, err := crdt.NewPubSubBroadcaster(ctx, ps, CRCLS_NS)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	dag, err := ipfslite.New(ctx, d, nil, h, dht, nil)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	evtStream := make(chan EntryEvent)

	// copts := crdt.Options{
	// 	Logger:              log,
	// 	RebroadcastInterval: time.Second * 10,
	// 	NumWorkers:          4,
	// 	DAGSyncerTimeout:    time.Second * 10,
	// 	MaxBatchDeltaSize:   100 * 1024,
	// 	RepairInterval:      time.Second * 30,
	// 	MultiHeadProcessing: true,
	// }
	copts := crdt.DefaultOptions()
	copts.Logger = log
	copts.RebroadcastInterval = time.Second * 3
	copts.DAGSyncerTimeout = time.Second * 3
	copts.PutHook = func(key ipfsDs.Key, value []byte) {
		ee := EntryEvent{key, value}
		evtStream <- ee
	}

	c, err := crdt.New(d, ipfsDs.NewKey(CRCLS_NS), dag, bcast, copts)
	if err != nil {
		log.Fatal(err)
	}

	ds := &Datastore{
		Badger:      d,
		EventStream: evtStream,
		crdt:        c,
	}

	internalDs = ds

	return ds
}

func (ds *Datastore) Put(ctx context.Context, key ipfsDs.Key, value []byte) error {
	return ds.crdt.Put(ctx, key, value)
}
func (ds *Datastore) Get(ctx context.Context, key ipfsDs.Key) ([]byte, error) {
	return ds.crdt.Get(ctx, key)
}
func (ds *Datastore) Delete(ctx context.Context, key ipfsDs.Key) error {
	return ds.crdt.Delete(ctx, key)
}
func (ds *Datastore) Close() error {
	return ds.crdt.Close()
}
func (ds *Datastore) Stats() crdt.Stats {
	return ds.crdt.InternalStats()
}
func (ds *Datastore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	return ds.crdt.Query(ctx, q)
}

func Put(ctx context.Context, key ipfsDs.Key, value []byte) error {
	if internalDs == nil {
		log.Fatal("Database is not initialized")
	}
	return internalDs.Put(ctx, key, value)
}

func Get(ctx context.Context, key ipfsDs.Key) ([]byte, error) {
	if internalDs == nil {
		log.Fatal("Database is not initialized")
	}
	return internalDs.Get(ctx, key)
}

func Query(ctx context.Context, q query.Query) (query.Results, error) {
	if internalDs == nil {
		log.Fatal("Database is not initialized")
	}
	return internalDs.crdt.Query(ctx, q)
}
