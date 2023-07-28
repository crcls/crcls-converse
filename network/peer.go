package network

import (
	"crypto/ecdsa"
	"fmt"

	dsecp "github.com/decred/dcrd/dcrec/secp256k1/v4"
	esecp "github.com/ethereum/go-ethereum/crypto/secp256k1"
	lp2pc "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func PeerIdToPublicKey(pid peer.ID) (*ecdsa.PublicKey, error) {
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		return nil, err
	}

	cpk, err := lp2pc.PubKeyToStdKey(pubKey)
	if err != nil {
		return nil, err
	}

	pk, ok := cpk.(*lp2pc.Secp256k1PublicKey)
	if !ok {
		return nil, fmt.Errorf("pk is not Secp256k1PublicKey")
	}

	x := (*dsecp.PublicKey)(pk).X()
	y := (*dsecp.PublicKey)(pk).Y()

	return &ecdsa.PublicKey{
		Curve: esecp.S256(),
		X:     x,
		Y:     y,
	}, nil
}
