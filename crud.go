package main

import (
	"context"
	"crcls-converse/logger"
	"encoding/json"
	"strings"

	ipfsDs "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

func (crcls *CRCLS) CreateCircle(ctx context.Context, data []byte) error {
	c := &Circle{}
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}

	id := c.Id
	c.Author = crcls.Account.Wallet.Address

	// TODO: validate the id
	// TODO: mint NFT

	data, err := json.Marshal(&c)
	if err != nil {
		return err
	}

	key := ipfsDs.NewKey("/circles/" + id)

	crcls.DS.Put(ctx, key, data)

	return nil
}

type FilterKeyIncludes struct {
	Matches []string
}

func (f FilterKeyIncludes) Filter(e query.Entry) bool {
	for _, match := range f.Matches {
		if found := strings.HasPrefix(e.Key, "/circles/"+match); found {
			return true
		}
	}
	return false
}

func (f FilterKeyIncludes) String() string {
	return "FilterKeyIncludes"
}

func (crcls *CRCLS) ListCircles(ctx context.Context) ([]*Circle, error) {
	log := logger.GetLogger()
	circles := make([]*Circle, 0)

	q := query.Query{
		Filters:  []query.Filter{FilterKeyIncludes{crcls.Account.Circles}},
		KeysOnly: false,
	}

	results, err := crcls.DS.Query(ctx, q)
	if err != nil {
		log.Debug(err)
		return circles, err
	}
	defer results.Close()

	for res := range results.Next() {
		circle := &Circle{}
		if err := json.Unmarshal(res.Value, circle); err != nil {
			log.Error(err)
			continue
		}

		circles = append(circles, circle)
	}

	return circles, nil
}
