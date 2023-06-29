package inout

import "fmt"

// Commands

// /list peers|rooms|members
// /join {room}
// /leave {room}
// /reply {member id}? - sends a message to the channel (add member id for direct reply)

const (
	LIST  = "list"
	JOIN  = "join"
	LEAVE = "leave"
	REPLY = "reply"
)

var CMDS = [4]string{LIST, JOIN, LEAVE, REPLY}

type InputCommand struct {
	Type string
	Data []byte
}

func parseCommand(in []byte) (*InputCommand, error) {
	if in[0] != '/' {
		return nil, fmt.Errorf("Malformed command. Must start with '/'")
	}

	if len(in) == 1 {
		return nil, fmt.Errorf("Empty command. '%s'", string(in))
	}

	var cmd []byte
	var data []byte
	for i, b := range in {
		if i == 0 {
			continue
		}
		if b == ' ' {
			data = in[i:]
			break
		}

		cmd = append(cmd, b)
	}

	found := false
	for _, c := range CMDS {
		if c == string(cmd) {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("Command not found. %s", string(cmd))
	}

	ic := &InputCommand{
		Type: string(cmd),
		Data: data,
	}

	return ic, nil
}
