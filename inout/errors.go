package inout

import "encoding/json"

type ErrorMessage struct {
	Type    string `json:"type"`
	message string
}

type ChannelError struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Error   error  `json:"error"`
}

func EmitChannelError(err error) {
	data, merr := json.Marshal(&ChannelError{
		Type:  "error",
		Error: err,
	})
	if merr != nil {
		log.Fatal(merr)
	}

	internalIO.Write(data)
}
