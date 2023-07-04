package account

import (
	"crcls-converse/config"
	"crcls-converse/evm"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crypto/rand"
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
)

var log = logger.GetLogger()

type Account struct {
	config *config.Config
	HostPk crypto.PrivKey
	IP     []byte
	Wallet *evm.Wallet
	Member *Member
}

func (a *Account) CreateWallet() *evm.Wallet {
	wallet := evm.New(a.config)
	a.Wallet = wallet

	return wallet
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
	hpk := GetIdentity(filepath.Join(conf.CrclsDir, "host_keyfile.pem"))

	ip, err := getIpAddress()
	if err != nil {
		log.Fatal(err)
	}

	account := &Account{
		config: conf,
		HostPk: hpk,
		IP:     []byte(ip.IP),
	}

	// TODO: support more chains
	privKey, err := evm.LoadAccount(filepath.Join(conf.CrclsDir, "wallet_pk"))
	if err != nil {
		evt := inout.NoAccountMessage{Type: "No account"}

		data, err := json.Marshal(evt)
		if err != nil {
			log.Fatal(err)
		}

		io.Write(data)
	} else {
		wallet := evm.NewWallet(privKey)

		account.Wallet = wallet
	}

	return account
}
