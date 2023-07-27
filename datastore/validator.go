package datastore

import (
	"context"
	"crcls-converse/config"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	badger "github.com/ipfs/go-ds-badger3"
)

type ValidatorDatastore struct {
	ds *badger.Datastore
}

func (v *ValidatorDatastore) Put(ctx context.Context, key datastore.Key, value []byte) error {
	return v.ds.Put(ctx, key, value)
}

func (v *ValidatorDatastore) Get(ctx context.Context, key datastore.Key) (value []byte, err error) {
	return v.ds.Get(ctx, key)
}

func (v *ValidatorDatastore) Has(ctx context.Context, key datastore.Key) (exists bool, err error) {
	return v.ds.Has(ctx, key)
}

func (v *ValidatorDatastore) Delete(ctx context.Context, key datastore.Key) error {
	return nil
}

func (v *ValidatorDatastore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	return v.ds.Query(ctx, q)
}

func (v *ValidatorDatastore) Sync(ctx context.Context, prefix datastore.Key) error {
	return v.ds.Sync(ctx, prefix)
}

func (v *ValidatorDatastore) Close() error {
	return v.ds.Close()
}

func (v *ValidatorDatastore) GetSize(ctx context.Context, key datastore.Key) (size int, err error) {
	return v.ds.GetSize(ctx, key)
}

func (v *ValidatorDatastore) Batch(ctx context.Context) (datastore.Batch, error) {
	batch, err := v.ds.Batch(ctx)
	if err != nil {
		return nil, err
	}
	return &ValidatorBatch{batch: batch}, nil
}

type ValidatorBatch struct {
	batch datastore.Batch
}

func (b *ValidatorBatch) Put(ctx context.Context, key datastore.Key, value []byte) error {
	// fmt.Printf("\nBatch: key: %s value %s\n\n", key, string(value))
	return b.batch.Put(ctx, key, value)
}

func (b *ValidatorBatch) Delete(ctx context.Context, key datastore.Key) error {
	return b.batch.Delete(ctx, key)
}

func (b *ValidatorBatch) Commit(ctx context.Context) error {
	return b.batch.Commit(ctx)
}

func NewValidatorDatastore(conf *config.Config) (*ValidatorDatastore, error) {
	ds, err := badger.NewDatastore(filepath.Join(conf.CrclsDir, STORE_DIR), &badger.DefaultOptions)
	if err != nil {
		return nil, err
	}

	return &ValidatorDatastore{
		ds: ds,
	}, nil
}
