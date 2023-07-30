package inout

import (
	"crcls-converse/logger"
	"encoding/json"
)

type ErrorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type KeyNotFoundError struct{}

func (e *KeyNotFoundError) Error() string {
	return "Keyfile not found."
}

func EmitError(err error) {
	log := logger.GetLogger()
	data, merr := json.Marshal(&ErrorMessage{
		Type:    "error",
		Message: err.Error(),
	})
	if merr != nil {
		log.Fatal(merr)
	}

	internalIO.Write(data)
}
