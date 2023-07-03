package inout

import (
	"encoding/json"
)

type ErrorMessage struct {
	Type    string `json:"type"`
	Message string
}

func EmitError(err error) {
	data, merr := json.Marshal(&ErrorMessage{
		Type:    "error",
		Message: err.Error(),
	})
	if merr != nil {
		log.Fatal(merr)
	}

	internalIO.Write(data)
}
