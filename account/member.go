package account

import (
	"context"
	"crcls-converse/datastore"
	"encoding/json"
	"fmt"
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
	key      ipfsDs.Key
}

/**
 * Saves the Member struct to the Datastore
 */
func NewMember(ctx context.Context, a *Account, m *Member) error {
	if a.Wallet == nil || m == nil {
		return fmt.Errorf("Members must connect a wallet.")
	}

	m.Address = a.Wallet.Address
	m.Channels = []string{"global"}

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	fmt.Printf("Data: %s\n", string(data))

	prettyAddress, err := a.Wallet.Address.MarshalText()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nPretty Address: %v\n\n", string(prettyAddress))

	key := ipfsDs.KeyWithNamespaces([]string{"members", string(prettyAddress)})
	fmt.Printf("Key: %s\n", key.String())

	// ctxTO, cancel := context.WithTimeout(ctx, time.Second*5)
	// defer cancel()

	if err = datastore.Put(ctx, key, data); err != nil {
		return err
	}

	fmt.Println("Put complete.")

	return nil
}

func GetMember(ctx context.Context, address common.Address) (*Member, error) {
	ctxTO, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	prettyAddr, err := address.MarshalText()
	if err != nil {
		return nil, err
	}

	key := ipfsDs.KeyWithNamespaces([]string{"members", string(prettyAddr)})

	data, err := datastore.Get(ctxTO, key)
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
