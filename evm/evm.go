package evm

import (
	"crcls-converse/inout"
	"crypto/ecdsa"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tyler-smith/go-bip39"
)

func GenerateAccount(keypath string) (*ecdsa.PrivateKey, error) {
	privKey, err := crypto.GenerateKey()

	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	keyfile, err := os.Create(keypath)
	if err != nil {
		return nil, err
	}
	keyfile.Close()

	if err := crypto.SaveECDSA(keypath, privKey); err != nil {
		return nil, err
	}

	return privKey, nil
}

func GenerateSeedPhrase(pk *ecdsa.PrivateKey) (string, error) {
	return bip39.NewMnemonic(crypto.FromECDSA(pk))
}

func LoadAccount(keypath string) (*ecdsa.PrivateKey, error) {
	if _, err := os.Stat(keypath); err != nil {
		if os.IsNotExist(err) {
			return nil, &inout.KeyNotFoundError{}
		}

		return nil, err
	}

	return crypto.LoadECDSA(keypath)
}

// func RecoverAccount(c *config.Config) (*Account, error) {
// 	var privKey *ecdsa.PrivateKey
// 	var err error
// 	if c.PrivateKey != "" {
// 		privKey, err = crypto.HexToECDSA(c.PrivateKey)
// 		if err != nil {
// 			fmt.Println("Failed to parse priv key")
// 			return nil, err
// 		}
// 	} else if c.RecoveryPhrase != "" {
// 		seed, err := bip39.NewSeedWithErrorChecking(c.RecoveryPhrase, "")
// 		if err != nil {
// 			return nil, err
// 		}
// 		masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
// 		if err != nil {
// 			return nil, err
// 		}
// 		privKey, err = wallet.DefaultDerivationPath.Derive(masterKey)
// 		if err != nil {
// 			return nil, err
// 		}
// 	} else {
// 		return nil, fmt.Errorf("Private key or Recovery phrase missing")
// 	}

// 	keyfile, err := os.Create(filepath.Join(c.HomeDir, ".bui", "keyfile"))
// 	if err != nil {
// 		fmt.Println("file error")
// 		return nil, err
// 	}
// 	keyfile.Close()

// 	if err := crypto.SaveECDSA(filepath.Join(c.HomeDir, ".bui/keyfile"), privKey); err != nil {
// 		return nil, err
// 	}
// 	wallet := wallet.NewKey(privKey)

// 	client, err := jsonrpc.NewClient(c.ProviderURL)
// 	if err != nil {
// 		return nil, err
// 	}

// 	ip, err := getIpAddress()
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &Account{
// 		Address: wallet.Address(),
// 		Client:  client,
// 		IP:      []byte(ip.IP),
// 		Wallet:  wallet,
// 	}, nil
// }
