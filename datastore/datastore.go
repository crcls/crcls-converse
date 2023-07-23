package datastore

import (
	"context"
	"crcls-converse/config"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crypto/ecdsa"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	ipfsDs "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"

	// "github.com/ipfs/go-datastore/query"
	badger "github.com/ipfs/go-ds-badger3"
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
	Badger      *badger.Datastore
	EventStream chan EntryEvent
	crdt        *crdt.Datastore
	log         *logging.ZapEventLogger
	pubsub      *PubSubBroadcaster
}

var internalDs *Datastore

// TODO: need a home dot directory for these things
const STORE_DIR = "ds"
const CRCLS_NS = "/crcls/datastore/1.0.0"

func NewDatastore(ctx context.Context, conf *config.Config, h *host.Host, ps *pubsub.PubSub, dht *kaddht.IpfsDHT) *Datastore {
	log := logger.GetLogger()
	// opts := badger.Options{
	// 	GcDiscardRatio: 0,
	// 	GcInterval:     0,
	// }
	d, err := badger.NewDatastore(filepath.Join(conf.CrclsDir, STORE_DIR), &badger.DefaultOptions)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	bcast, err := NewPubSubBroadcaster(ctx, ps, CRCLS_NS)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	dag, err := ipfslite.New(ctx, d, nil, *h, dht, nil)
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
	copts.RebroadcastInterval = time.Second * 33
	copts.DAGSyncerTimeout = time.Second * 33
	copts.PutHook = func(key ipfsDs.Key, value []byte) {
		// TODO: moar validation
		d.Put(context.Background(), key, value)
		ee := EntryEvent{key, value}
		evtStream <- ee
	}

	c, err := crdt.New(ipfsDs.NewMapDatastore(), ipfsDs.NewKey(CRCLS_NS), dag, bcast, copts)
	if err != nil {
		log.Fatal(err)
	}

	ds := &Datastore{
		Badger:      d,
		EventStream: evtStream,
		crdt:        c,
		log:         log,
		pubsub:      bcast,
	}

	internalDs = ds

	return ds
}

func (ds *Datastore) Authenticate(pk *ecdsa.PrivateKey) {
	ds.PK = pk
	ds.pubsub.Authenticate(pk)
}

func (ds *Datastore) Put(ctx context.Context, key ipfsDs.Key, value []byte) error {
	if ds.PK == nil {
		return fmt.Errorf("Not authenticated")
	}

	return ds.crdt.Put(ctx, key, value)
}
func (ds *Datastore) Get(ctx context.Context, key ipfsDs.Key) ([]byte, error) {
	return ds.Badger.Get(ctx, key)
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
	return ds.Badger.Query(ctx, q)
}

func Put(ctx context.Context, key ipfsDs.Key, value []byte) error {
	if internalDs == nil {
		internalDs.log.Fatal("Database is not initialized")
	}
	return internalDs.Put(ctx, key, value)
}

func Get(ctx context.Context, key ipfsDs.Key) ([]byte, error) {
	if internalDs == nil {
		internalDs.log.Fatal("Database is not initialized")
	}
	return internalDs.Get(ctx, key)
}

func Query(ctx context.Context, q query.Query) (query.Results, error) {
	if internalDs == nil {
		internalDs.log.Fatal("Database is not initialized")
	}
	return internalDs.Query(ctx, q)
}
