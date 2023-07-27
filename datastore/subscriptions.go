package datastore

import (
	"crcls-converse/inout"
	"encoding/json"
	"fmt"

	ipfsDs "github.com/ipfs/go-datastore"
)

type Subscriptions struct {
	subs map[ipfsDs.Key]func(*inout.Message)
}

func (sub *Subscriptions) Propagate(k ipfsDs.Key, v []byte) {
	msg := inout.Message{}
	if err := json.Unmarshal(v, &msg); err != nil {
		inout.EmitError(err)
		return
	}

	for key, handleSub := range sub.subs {
		if key.IsAncestorOf(k) || key.IsDescendantOf(key) {
			fmt.Printf("Sending msg: %v\n", msg)
			handleSub(&msg)
		}
	}
}

func (sub *Subscriptions) Subscribe(prefix ipfsDs.Key) chan *inout.Message {
	ch := make(chan *inout.Message)

	sub.subs[prefix] = func(msg *inout.Message) {
		ch <- msg
	}

	return ch
}

func (sub *Subscriptions) Unsubscribe(prefix ipfsDs.Key) error {
	if _, ok := sub.subs[prefix]; !ok {
		return fmt.Errorf("Subscription not found")
	}

	delete(sub.subs, prefix)
	return nil
}

func NewSubscriptions() *Subscriptions {
	return &Subscriptions{
		subs: map[ipfsDs.Key]func(*inout.Message){},
	}
}
