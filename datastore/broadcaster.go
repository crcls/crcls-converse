package datastore

import (
	"context"
	"crcls-converse/pb"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	crdt "github.com/ipfs/go-ds-crdt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"
)

type ECDSASignature struct {
	R, S *big.Int
}

func ecdsaPublicKeyToProto(key *ecdsa.PublicKey) (*pb.ECDSAPublicKey, error) {
	curveName := "secp256k1" // Hard-coding the curve name

	// Directly convert big.Int to bytes
	x := key.X.Bytes()
	y := key.Y.Bytes()

	return &pb.ECDSAPublicKey{
		Curve: curveName,
		X:     x,
		Y:     y,
	}, nil
}

func protoToEcdsaPublicKey(key *pb.ECDSAPublicKey) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch key.Curve {
	case "secp256k1":
		curve = secp256k1.S256()
	default:
		return nil, fmt.Errorf("unknown elliptic curve %s", key.Curve)
	}

	x := new(big.Int).SetBytes(key.X)
	y := new(big.Int).SetBytes(key.Y)

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}, nil
}

// PubSubBroadcaster implements a Broadcaster using libp2p PubSub.
type PubSubBroadcaster struct {
	PK    *ecdsa.PrivateKey
	ctx   context.Context
	psub  *pubsub.PubSub
	topic *pubsub.Topic
	subs  *pubsub.Subscription
}

// NewPubSubBroadcaster returns a new broadcaster using the given PubSub and
// a topic to subscribe/broadcast to. The given context can be used to cancel
// the broadcaster.
// Please register any topic validators before creating the Broadcaster.
//
// The broadcaster can be shut down by cancelling the given context.
// This must be done before Closing the crdt.Datastore, otherwise things
// may hang.
func NewPubSubBroadcaster(ctx context.Context, psub *pubsub.PubSub, topic string) (*PubSubBroadcaster, error) {
	psubTopic, err := psub.Join(topic)
	if err != nil {
		return nil, err
	}

	subs, err := psubTopic.Subscribe()
	if err != nil {
		return nil, err
	}

	go func(ctx context.Context, subs *pubsub.Subscription) {
		<-ctx.Done()
		subs.Cancel()
	}(ctx, subs)

	return &PubSubBroadcaster{
		ctx:   ctx,
		psub:  psub,
		topic: psubTopic,
		subs:  subs,
	}, nil
}

func (pbc *PubSubBroadcaster) Authenticate(pk *ecdsa.PrivateKey) {
	pbc.PK = pk
}

// Broadcast publishes some data.
func (pbc *PubSubBroadcaster) Broadcast(data []byte) error {
	if pbc.PK == nil {
		return fmt.Errorf("Not authenticated")
	}

	hashedMessage := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, pbc.PK, hashedMessage[:])
	if err != nil {
		return err
	}
	// Create a signature struct
	signature := ECDSASignature{R: r, S: s}

	// Marshal the signature to ASN.1
	sig, err := asn1.Marshal(signature)
	if err != nil {
		return err
	}

	pubKey, err := ecdsaPublicKeyToProto(&pbc.PK.PublicKey)
	if err != nil {
		return err
	}

	broadcast := &pb.CRDTBroadcast{Message: data, Signature: sig, Key: pubKey}
	msg, err := proto.Marshal(broadcast)

	return pbc.topic.Publish(pbc.ctx, msg)
}

// Next returns published data.
func (pbc *PubSubBroadcaster) Next() ([]byte, error) {
	var msg *pubsub.Message
	var err error

	select {
	case <-pbc.ctx.Done():
		return nil, crdt.ErrNoMoreBroadcast
	default:
	}

	msg, err = pbc.subs.Next(pbc.ctx)
	if err != nil {
		if strings.Contains(err.Error(), "subscription cancelled") ||
			strings.Contains(err.Error(), "context") {
			return nil, crdt.ErrNoMoreBroadcast
		}
		return nil, err
	}

	broadcast := &pb.CRDTBroadcast{}
	if err := proto.Unmarshal(msg.GetData(), broadcast); err != nil {
		return nil, err
	}

	hashedMessage := sha256.Sum256(broadcast.Message)

	pubkey, err := protoToEcdsaPublicKey(broadcast.Key)
	if err != nil {
		return nil, err
	}

	var sig ECDSASignature
	_, err = asn1.Unmarshal(broadcast.Signature, &sig)
	if err != nil {
		return nil, err
	}

	if valid := ecdsa.Verify(pubkey, hashedMessage[:], sig.R, sig.S); valid {
		return broadcast.Message, nil
	}

	return nil, fmt.Errorf("Signature verification failed")
}
