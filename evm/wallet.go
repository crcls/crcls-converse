package evm

import (
	"context"
	"crcls-converse/config"
	"crcls-converse/logger"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	logging "github.com/ipfs/go-log/v2"
)

var RPC = os.Getenv("POLYGON_RPC")

type Wallet struct {
	Address    common.Address
	AuthSig    *AuthSig
	Client     *rpc.Client
	PrivKey    *ecdsa.PrivateKey
	PubKey     ecdsa.PublicKey
	SeedPhrase string
	log        *logging.ZapEventLogger
}

func (w *Wallet) Balance() (*big.Int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	var out string
	if err := w.Client.CallContext(ctx, &out, "eth_getBalance", w.Address, Latest.Location()); err != nil {
		return nil, err
	}
	b, ok := new(big.Int).SetString(out[2:], 16)
	if !ok {
		return nil, fmt.Errorf("failed to convert to big.int")
	}

	return b, nil
}

func WeiToEth(wei *big.Int) *big.Float {
	return new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(1e18))
}

func NewWallet(pk *ecdsa.PrivateKey) *Wallet {
	log := logger.GetLogger()
	seed, err := GenerateSeedPhrase(pk)
	if err != nil {
		log.Fatal(err)
	}

	client, err := rpc.Dial(RPC)
	if err != nil {
		log.Fatal(err)
	}

	return &Wallet{
		Address:    crypto.PubkeyToAddress(pk.PublicKey),
		Client:     client,
		PrivKey:    pk,
		PubKey:     pk.PublicKey,
		SeedPhrase: seed,
		log:        log,
	}
}

func New(conf *config.Config) *Wallet {
	log := logger.GetLogger()
	priv, err := GenerateAccount(filepath.Join(conf.CrclsDir, "wallet_pk"))
	if err != nil {
		log.Fatal(err)
	}

	wallet := NewWallet(priv)

	return wallet
}
