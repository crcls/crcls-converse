package datastore

import (
	"context"
	"crcls-converse/config"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"fmt"
	"path/filepath"

	ipfsDs "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	badger "github.com/ipfs/go-ds-badger3"
)

type LocalStore struct {
	ds *badger.Datastore
}

func (store *LocalStore) ValidatedSync(key ipfsDs.Key, value []byte) {
	log := logger.GetLogger()

	fmt.Printf("\n----------------\n%s %s\n", key.String(), string(value))

	// TODO: moar validation
	if err := store.ds.Put(context.Background(), key, value); err != nil {
		log.Errorf("%v", err)
	}

	if _, err := store.ds.Has(context.Background(), key); err != nil {
		log.Errorf("%v", err)
	}

	fmt.Printf("DB has %s\n", key.String())

	// result, err := store.ds.Get(context.Background(), key)
	// if err != nil {
	// 	log.Errorf("%v", err)
	// }

	// fmt.Printf("%v\n", result)
	fmt.Printf("-----------------\n\n")
}

func (store *LocalStore) Get(ctx context.Context, key ipfsDs.Key) ([]byte, error) {
	return store.ds.Get(ctx, key)
}
func (store *LocalStore) Delete(ctx context.Context, key ipfsDs.Key) error {
	return store.ds.Delete(ctx, key)
}
func (store *LocalStore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	return store.ds.Query(ctx, q)
}

func NewLocalStore(conf *config.Config) *LocalStore {
	ds, err := badger.NewDatastore(filepath.Join(conf.CrclsDir, STORE_DIR), &badger.DefaultOptions)
	if err != nil {
		inout.EmitError(err)
		return nil
	}

	ls := &LocalStore{
		ds: ds,
	}

	return ls
}
