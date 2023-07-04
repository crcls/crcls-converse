package evm

import (
	"fmt"
)

type BlockNumber int

const (
	Latest   BlockNumber = -1
	Earliest BlockNumber = -2
	Pending  BlockNumber = -3
)

func (b BlockNumber) Location() string {
	return b.String()
}

func (b BlockNumber) String() string {
	switch b {
	case Latest:
		return "latest"
	case Earliest:
		return "earliest"
	case Pending:
		return "pending"
	}
	if b < 0 {
		panic("internal. blocknumber is negative")
	}
	return fmt.Sprintf("0x%x", uint64(b))
}
