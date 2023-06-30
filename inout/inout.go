package inout

import (
	"bufio"
	"crcls-converse/logger"
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

var internalIO *IO

func (io *IO) Write(msg []byte) {
	io.stdout.Write(append(msg, []byte("\n")...))
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

	internalIO = &IO{stdin: stdin, stdout: stdout, InputChan: ic}

	go internalIO.Read()

	return internalIO
}
