package inout

import (
	"bufio"
	"crcls-converse/logger"
	"encoding/json"
	"os"
)

var log = logger.GetLogger()

type Event struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type IO struct {
	stdout    *bufio.Writer
	stdin     *bufio.Reader
	InputChan chan *InputCommand
}

func (io *IO) Write(eventType string, msg []byte) {
	data, err := json.Marshal(Event{Type: eventType, Message: string(msg)})

	if err != nil {
		log.Error(err)
		return
	}

	io.stdout.Write(data)
	io.stdout.Write([]byte("\n"))
	if err := io.stdout.Flush(); err != nil {
		log.Error(err)
	}
}

func (io *IO) Read() {
	for {
		data, err := io.stdin.ReadBytes('\n')
		if err != nil {
			log.Error(err)
		}

		// Truncate the trailing return char
		data = data[:len(data)-1]

		if len(data) == 0 {
			log.Error("Empty command")
			continue
		}

		if ic, err := parseCommand(data); err == nil {
			log.Debug(ic)
			io.InputChan <- ic
		} else {
			log.Error(err)
		}
	}
}

func Connect() *IO {
	stdin := bufio.NewReader(os.Stdin)
	stdout := bufio.NewWriter(os.Stdout)
	ic := make(chan *InputCommand)

	io := &IO{stdin: stdin, stdout: stdout, InputChan: ic}

	go io.Read()

	return io
}
