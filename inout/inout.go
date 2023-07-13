package inout

import (
	"bufio"
	"crcls-converse/logger"
	logging "github.com/ipfs/go-log/v2"
	"os"
)

type Event struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type IO struct {
	stdout    *bufio.Writer
	stdin     *bufio.Reader
	InputChan chan *InputCommand
	log       *logging.ZapEventLogger
}

var internalIO *IO

func (io *IO) Write(msg []byte) {
	io.stdout.Write(append(msg, []byte("\n")...))
	if err := io.stdout.Flush(); err != nil {
		io.log.Error(err)
	}
}

func (io *IO) Read() {
	for {
		data, err := io.stdin.ReadBytes('\n')
		if err != nil {
			io.log.Error(err)
		}

		// Truncate the trailing return char
		data = data[:len(data)-1]

		if len(data) == 0 {
			io.log.Error("Empty command")
			continue
		}

		if ic, err := parseCommand(data); err == nil {
			io.InputChan <- ic
		} else {
			io.log.Error(err)
		}
	}
}

func Connect() *IO {
	log := logger.GetLogger()
	stdin := bufio.NewReader(os.Stdin)
	stdout := bufio.NewWriter(os.Stdout)
	ic := make(chan *InputCommand)

	internalIO = &IO{stdin: stdin, stdout: stdout, InputChan: ic, log: log}

	go internalIO.Read()

	return internalIO
}
