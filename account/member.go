package account

import (
	"context"
	"crcls-converse/datastore"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ipfsDs "github.com/ipfs/go-datastore"
)

type Member struct {
	Address  common.Address `json:"address"`
	Bio      string         `json:"bio"`
	Channels []string       `json:"channels"`
	Handle   string         `json:"handle"`
	PFP      string         `json:"pfp"`
	Balance  *big.Int       `json:"balance"`
	key      ipfsDs.Key
}

/**
 * Saves the Member struct to the Datastore
 */
func NewMember(ctx context.Context, a *Account, m *Member, ds *datastore.Datastore) error {
	log := logger.GetLogger()
	if a.Wallet == nil || m == nil {
		return fmt.Errorf("Members must connect a wallet.")
	}

	m.Address = a.Wallet.Address
	m.Channels = []string{"global"}

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	prettyAddress, err := a.Wallet.Address.MarshalText()
	if err != nil {
		return err
	}

	go func() {
		select {
		case <-ds.EventStream:
			msg := inout.MemberCreateMessage{
				Type:    "memberCreate",
				Handle:  m.Handle,
				PFP:     m.PFP,
				Address: string(prettyAddress),
			}

			data, err := json.Marshal(&msg)
			if err != nil {
				log.Fatal(err)
			}

			inout.EmitMessage(data)
		case <-ctx.Done():
			return
		}
	}()

	key := ipfsDs.NewKey("members").Instance(string(prettyAddress))
	if err = ds.Put(ctx, key, data); err != nil {
		return err
	}

	return nil
}

func GetMember(ctx context.Context, address common.Address, ds *datastore.Datastore) (*Member, error) {
	ctxTO, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	prettyAddr, err := address.MarshalText()
	if err != nil {
		return nil, err
	}

	key := ipfsDs.NewKey("members").Instance(string(prettyAddr))

	data, err := ds.Get(ctxTO, key)
	if err != nil {
		return nil, err
	}

	member := &Member{}

	if err := json.Unmarshal(data, member); err != nil {
		return nil, err
	}

	member.key = key

	return member, nil
}
