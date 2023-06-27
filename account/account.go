package account

import (
	"crypto/rand"
	"io/ioutil"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
)

type Account struct {
	PK crypto.PrivKey
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

		// save the private key to file
		keyBytes, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			panic(err)
		}
		// TODO: make sure the directory exists
		err = ioutil.WriteFile(keyFile, keyBytes, 0600)
		if err != nil {
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

func New() *Account {
	// TODO: find a better place for the keyfile
	pk := GetIdentity("./crcls_keyfile.pem")

	return &Account{pk}
}
