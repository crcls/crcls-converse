package datastore

import (
	"context"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crcls-converse/network"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	ipfsDs "github.com/ipfs/go-datastore"
	// "github.com/ipfs/go-datastore/query"
	badger "github.com/ipfs/go-ds-badger3"
	crdt "github.com/ipfs/go-ds-crdt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type Datastore struct {
	ds   *badger.Datastore
	crdt *crdt.Datastore
}

var log = logger.GetLogger()

// TODO: need a home dot directory for these things
const STORE_DIR = "./crcls-ds"
const CRCLS_NS = "/crcls/datastore/1.0.0"

func NewDatastore(ctx context.Context, net *network.Network) *Datastore {
	// opts := badger.Options{
	// 	GcDiscardRatio: 0,
	// 	GcInterval:     0,
	// }
	d, err := badger.NewDatastore(STORE_DIR, nil)
	if err != nil {
		inout.EmitChannelError(err)
		return nil
	}

	ps, err := pubsub.NewGossipSub(ctx, net.Host)
	if err != nil {
		inout.EmitChannelError(err)
		return nil
	}

	bcast, err := crdt.NewPubSubBroadcaster(ctx, ps, CRCLS_NS)
	if err != nil {
		inout.EmitChannelError(err)
		return nil
	}

	dag, err := ipfslite.New(ctx, d, nil, net.Host, net.DHT, nil)
	if err != nil {
		inout.EmitChannelError(err)
		return nil
	}

	copts := crdt.Options{
		Logger:              log,
		RebroadcastInterval: time.Second * 10,
		NumWorkers:          4,
		DAGSyncerTimeout:    time.Second * 10,
		MaxBatchDeltaSize:   100 * 1024,
		RepairInterval:      time.Second * 30,
		MultiHeadProcessing: true,
	}

	c, err := crdt.New(d, ipfsDs.NewKey(CRCLS_NS), dag, bcast, &copts)
	if err != nil {
		log.Fatal(err)
	}

	return &Datastore{
		ds:   d,
		crdt: c,
	}
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