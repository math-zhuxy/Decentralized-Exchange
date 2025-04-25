package global2

import (
	"github.com/ethereum/go-ethereum/common"
	"sync"
)

var StateDB interface{}
var Lock sync.Mutex
var Root  common.Hash