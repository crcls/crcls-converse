package evm

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type AuthSig struct {
	Sig           string `json:"sig"`
	DerivedVia    string `json:"derivedVia"`
	SignedMessage string `json:"signedMessage"`
	Address       string `json:"address"`
}

const PREFIX_191 = "\x19Ethereum Signed Message:\n"

func EIP191(msg string) (msgBytes []byte) {
	msgLen := len(msg)
	msgBytes = append(msgBytes, []byte(PREFIX_191)...)
	msgBytes = append(msgBytes, []byte(strconv.FormatInt(int64(msgLen), 10))...)
	msgBytes = append(msgBytes, []byte(msg)...)
	return
}

func EIP4361(address common.Address, msg, chain, nonce, date string) string {
	return fmt.Sprintf(`CRCLS wants you to sign in with your Ethereum account:
%s

%s
URI: https://crcls.xyz
Version: 1
Chain ID: %s
Nonce: %s
Issued At: %s`, address, msg, chain, nonce, date)
}

func (w *Wallet) Siwe(chain, msg string) (*AuthSig, error) {
	date := time.Now()

	eip4361 := EIP4361(w.Address, msg, chain, strconv.FormatInt(date.Unix(), 10), date.Format(time.RFC3339))
	msgBytes := EIP191(eip4361)

	sig, err := crypto.Sign(crypto.Keccak256(msgBytes), w.PrivKey)
	if err != nil {
		return nil, err
	}

	authSig := &AuthSig{
		Address:       w.Address.String(),
		DerivedVia:    "ethgo.Key.SignMsg",
		SignedMessage: eip4361,
		Sig:           "0x" + hex.EncodeToString(sig),
	}

	return authSig, nil
}

func VerifyAddress(pubKey common.Address, signature, plaintext string) bool {
	sig, err := hex.DecodeString(signature[2:] /* Remove 0x */)
	if err != nil {
		log.Debug("AuthSig failed to hex decode sig", err)
		return false
	}

	// TODO: remember what this solves. Something to do with Lit...
	if sig[len(sig)-1] == 28 {
		sig[len(sig)-1] = 1
	}

	// fmt.Printf("%s\n", string(EIP191(plaintext)))

	return crypto.VerifySignature(pubKey.Bytes(), EIP191(plaintext), sig)
}
