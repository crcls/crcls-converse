package main

import "github.com/ethereum/go-ethereum/common"

type Circle struct {
	Author      common.Address `json:"author"`
	Description *string        `json:"description"`
	Id          string         `json:"id"`
	Logo        *string        `json:"logo"`
	Name        string         `json:"name"`
	Uri         string         `json:"uri"`
}
