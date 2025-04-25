package state

import (
	"github.com/ethereum/go-ethereum/common"
	"sync"
)

type OpCallRes struct {
	Journal []Wrapper
	Ret []byte
	Err string
	Gas uint64
}

type OpStaticCallRes struct {
	Ret []byte
	Err string
	Gas uint64
}
var OpCallResMap = make(map[string]OpCallRes)
var OpStaticCallResMap = make(map[string]OpStaticCallRes)
var OpCallResMapLock sync.Mutex
var OpStaticCallResMapLock sync.Mutex


var OpCreateResMapLock sync.Mutex
var OpCreateResMap=make(map[string]OpCreateRes)
type OpCreateRes struct {
	Addr common.Address
	Ret []byte
	Err string
	Id string
	Journal []Wrapper
}