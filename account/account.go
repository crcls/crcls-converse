package account

import (
	"crcls-converse/config"
	"crcls-converse/evm"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crypto/rand"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"

	logging "github.com/ipfs/go-log/v2"
)

type Account struct {
	HostPk crypto.PrivKey
	IP     []byte
	Wallet *evm.Wallet
	Member *Member
	config *config.Config
	log    *logging.ZapEventLogger
}

func (a *Account) CreateWallet() *evm.Wallet {
	wallet := evm.New(a.config)
	a.Wallet = wallet

	return wallet
}

func (a *Account) MarshalJSON() ([]byte, error) {
	data := struct {
		Address  string   `json:"address"`
		Balance  *big.Int `json:"balance"`
		Handle   string   `json:"handle"`
		Pfp      string   `json:"pfp"`
		Bio      string   `json:"bio"`
		Channels []string `json:"channels"`
	}{}

	if a.Wallet != nil {
		addr, err := a.Wallet.Address.MarshalText()
		if err != nil {
			addr = []byte(a.Wallet.Address.String())
		}

		balance, err := a.Wallet.Balance()
		if err != nil {
			a.log.Error(err)
		}

		data.Address = string(addr)
		data.Balance = balance
	}

	if a.Member != nil {
		data.Handle = a.Member.Handle
		data.Pfp = a.Member.PFP
		data.Bio = a.Member.Bio
		data.Channels = a.Member.Channels
	}

	if a.Wallet == nil && a.Member == nil {
		return json.Marshal(nil)
	}

	return json.Marshal(&data)
}

func getIpAddress() (*net.UDPAddr, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	ipAddress := conn.LocalAddr().(*net.UDPAddr)

	return ipAddress, nil
}

func GetIdentity(keyFile string) crypto.PrivKey {
	var priv crypto.PrivKey
	var err error
	if _, err = os.Stat(keyFile); os.IsNotExist(err) {
		// Key file does not exist, create a new one
		priv, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			panic(err)
		}

		keyBytes, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			panic(err)
		}

		if err = ioutil.WriteFile(keyFile, keyBytes, 0600); err != nil {
			panic(err)
		}
	} else {
		// Load key from file
		keyBytes, err := ioutil.ReadFile(keyFile)
		if err != nil {
			panic(err)
		}
		priv, err = crypto.UnmarshalPrivateKey(keyBytes)
		if err != nil {
			panic(err)
		}
	}

	return priv
}

func Load(conf *config.Config, io *inout.IO) *Account {
	log := logger.GetLogger()
	hpk := GetIdentity(filepath.Join(conf.CrclsDir, "host_keyfile.pem"))

	ip, err := getIpAddress()
	if err != nil {
		log.Fatal(err)
	}

	account := &Account{
		config: conf,
		HostPk: hpk,
		IP:     []byte(ip.IP),
		log:    log,
	}

	// TODO: support more chains
	privKey, err := evm.LoadAccount(filepath.Join(conf.CrclsDir, "wallet_pk"))
	if err == nil {
		account.Wallet = evm.NewWallet(privKey)
	}

	return account
}
