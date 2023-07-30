package main

import (
	"context"
	"crcls-converse/account"
	"crcls-converse/config"
	"crcls-converse/inout"
	"crcls-converse/logger"
	"crcls-converse/network"

	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/crypto"
)

type ReadyMessage struct {
	Type    string           `json:"type"`
	Status  string           `json:"status"`
	Account *account.Account `json:"account"`
	Circles []*Circle        `json:"circles"`
}

func emitReadyEvent(app *CRCLS) error {
	log := logger.GetLogger()
	circles, err := app.ListCircles(context.Background())
	if err != nil {
		log.Debug(err)
	}

	readyEvent, err := json.Marshal(&ReadyMessage{
		Type:    "ready",
		Status:  "connected",
		Account: app.Account,
		Circles: circles,
	})
	if err != nil {
		return err
	}
	app.IO.Write(readyEvent)

	return nil
}

func ensureInit() {
	if crcls == nil {
		inout.EmitError(fmt.Errorf("Not authorized."))
		os.Exit(1)
	}
}

func main() {
	ctx := context.Background()
	conf := config.New()
	io := inout.Connect()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	//-------------------------
	// 1. Try init CRCLS
	// 2. Success breaks first step
	// 3. Wait for create-account or add-account
	// 4. Init services
	// 5. Reply with data

	if err := LoadCRCLS(ctx, conf, io); err != nil {
		if _, ok := err.(*inout.KeyNotFoundError); !ok {
			inout.EmitError(err)
		}

		authCtx, authCancel := context.WithCancel(context.Background())
		done := false
		for !done {
			select {
			case cmd := <-io.InputChan:
				if cmd.Type != inout.ACCOUNT {
					inout.EmitError(fmt.Errorf("Not authorized."))
					break
				}

				subcmd, err := cmd.NextSubcommand()
				if err != nil {
					inout.EmitError(err)
					break
				} else if subcmd != inout.CREATE {
					inout.EmitError(fmt.Errorf("Must create an account."))
					break
				}

				if err := CreateCRCLS(authCtx, conf, io); err != nil {
					inout.EmitError(err)
					break
				}

				done = true
			case <-authCtx.Done():
				done = true
			case <-stop:
				authCancel()
				os.Exit(0)
			}
		}
	}

	emitReadyEvent(crcls)

	for {
		select {
		case cmd := <-io.InputChan:
			if cmd.Type == inout.READY {
				emitReadyEvent(crcls)
			} else if err := crcls.Run(ctx, cmd); err != nil {
				inout.EmitError(err)
			}
		case status := <-crcls.Net.StatusChan:
			if status.Error != nil {
				inout.EmitError(status.Error)
			} else {
				epk, err := network.PeerIdToPublicKey(status.Peer)
				if err != nil {
					inout.EmitError(err)
					break
				}

				addr, err := crypto.PubkeyToAddress(*epk).MarshalText()
				if err != nil {
					inout.EmitError(err)
					break
				}

				data, err := json.Marshal(inout.PeerMessage{Type: "peer", Connected: status.Connected, Id: string(addr)})
				if err != nil {
					inout.EmitError(err)
					break
				}

				io.Write(data)
			}
		case <-stop:
			os.Exit(0)
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
