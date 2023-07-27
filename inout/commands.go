package inout

import "fmt"

// Commands

// TODO: Organize this better
const (
	AUTH    = "auth"
	LIST    = "list"
	JOIN    = "join"
	LEAVE   = "leave"
	REPLY   = "reply"
	MEMBER  = "member"
	ACCOUNT = "account"
	READY   = "ready"
)

// sub-commands
const (
	PEERS    = "peers"
	CHANNELS = "channels"
	MEMBERS  = "members"
	MESSAGES = "messages"
	CREATE   = "create"
)

type CMD string

var CMDS = [7]CMD{LIST, JOIN, LEAVE, REPLY, MEMBER, ACCOUNT, READY}

type InputCommand struct {
	Type    string
	Data    []byte
	Current CMD
}

func (ic *InputCommand) NextSubcommand() (CMD, error) {
	next, rest, err := parseNext(ic.Data)
	if err != nil {
		return "", err
	}

	ic.Current = next
	ic.Data = rest

	return next, nil
}

func parseNext(in []byte) (CMD, []byte, error) {
	if len(in) == 0 {
		return "", nil, fmt.Errorf("Data empty.")
	}

	var next []byte
	var rest []byte
	for i, b := range in {
		if b == ' ' {
			rest = in[i:]
			break
		}

		next = append(next, b)
	}

	// Drop the leading space for rest.
	if len(rest) > 0 {
		rest = rest[1:]
	}

	return CMD(next), rest, nil
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
