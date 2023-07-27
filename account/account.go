package account

import (
	"crcls-converse/config"
	"crcls-converse/evm"
	"crcls-converse/logger"
	"encoding/json"
	"math/big"
	"net"
	"path/filepath"

	logging "github.com/ipfs/go-log/v2"
)

type Account struct {
	IP     []byte
	Wallet *evm.Wallet
	Member *Member
	log    *logging.ZapEventLogger
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

func Load(conf *config.Config) (*Account, error) {
	log := logger.GetLogger()
	// TODO: support more chains
	privKey, err := evm.LoadAccount(filepath.Join(conf.CrclsDir, "wallet_pk"))
	if err != nil {
		return nil, err
	}

	ip, err := getIpAddress()
	if err != nil {
		return nil, err
	}

	account := &Account{
		Wallet: evm.NewWallet(privKey),
		IP:     []byte(ip.IP),
		log:    log,
	}

	return account, nil
}

func New(conf *config.Config) (*Account, error) {
	log := logger.GetLogger()

	ip, err := getIpAddress()
	if err != nil {
		return nil, err
	}

	return &Account{
		Wallet: evm.New(conf),
		IP:     []byte(ip.IP),
		log:    log,
	}, nil
}
