package inout

import "fmt"

// Commands

// /list peers|channels|members
// /join {room}
// /leave {room}
// /reply {member id}? - sends a message to the channel (add member id for direct reply)

const (
	LIST  = "list"
	JOIN  = "join"
	LEAVE = "leave"
	REPLY = "reply"
)

// LIST subcommands
const (
	PEERS    = "peers"
	CHANNELS = "channels"
	MEMBERS  = "members"
)

type CMD string

var CMDS = [4]CMD{LIST, JOIN, LEAVE, REPLY}
var LIST_SUBS = [3]CMD{PEERS, CHANNELS, MEMBERS}

type InputCommand struct {
	Type    string
	Data    []byte
	Current CMD
}

func (ic *InputCommand) NextSubcomand() (CMD, error) {
	next, rest, err := parseNext(ic.Data)
	if err != nil {
		return "", err
	}

	ic.Current = next
	ic.Data = rest

	return next, nil
}

func parseNext(in []byte) (CMD, []byte, error) {
	if len(in) < 2 {
		return "", nil, fmt.Errorf("Data empty.")
	}

	var next []byte
	var rest []byte
	for i, b := range in {
		if b == ' ' || b == '\n' {
			rest = in[i:]
			break
		}

		next = append(next, b)
	}

	if len(rest) > 0 {
		rest = rest[1:]
	}

	return CMD(next), rest, nil // Drop the leading space for rest.
}

func parseCommand(in []byte) (*InputCommand, error) {
	if in[0] != '/' {
		return nil, fmt.Errorf("Malformed command. Must start with '/'")
	}

	cmd, data, err := parseNext(in[1:]) // Drop the '/' prefix
	if err != nil {
		return nil, fmt.Errorf("Empty command. '%s'", string(in))
	}

	found := false
	for _, c := range CMDS {
		if c == cmd {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("Command not found. %s", cmd)
	}

	ic := &InputCommand{
		Type:    string(cmd),
		Data:    data,
		Current: cmd,
	}

	return ic, nil
}
