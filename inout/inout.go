package inout

import (
	"bufio"
	"crcls-converse/logger"
	"encoding/json"
	"fmt"
	"os"
)

type IO struct {
	rw *bufio.ReadWriter
}

var internalIO IO
var log = logger.GetLogger()

func Connect() {
	stdin := bufio.NewReader(os.Stdin)
	stdout := bufio.NewWriter(os.Stdout)
	rw := bufio.NewReadWriter(stdin, stdout)

	internalIO = IO{rw}

	go readData(rw)
	go writeData(rw)
}

func Write(msg PeerMessage) {
	data, err := json.Marshal(msg)
	log.Debug("Writing: %s", string(data))

	if err != nil {
		log.Error(err)
		return
	}

	internalIO.rw.Write(data)
}

func writeData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func readData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}
