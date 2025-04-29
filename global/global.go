package global

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

var NodeAccount string
var NodeAccountMap map[uint]string
var NodeAccountMapLock sync.Mutex
var NodeAccChan chan uint = make(chan uint)
var AccBalanceMap map[string]uint64 = make(map[string]uint64)
var AccBalanceMapLock sync.Mutex

var AccCodeMap map[string][]byte = make(map[string][]byte)
var AccCodeMapLock sync.Mutex
var NodePublicKey string

//var CurChain *chain.BlockChain

var Ip_nodeTable map[uint64]map[uint64]string

var NodeID uint64
var ShardID uint64
var View uint64

var PartitionMap = make(map[string]uint64)
var Pmlock sync.RWMutex
var OpBalanceResMap = make(map[string]uint64)
var OpBalanceResMapLock sync.Mutex

var OpDelegatecallResMapLock sync.Mutex
var OpDelegatecallResMap = make(map[string]OpDelegatecallRes)

type OpDelegatecallRes struct {
	CodeHash common.Hash
	Code     []byte
}

var OpCallCodeResMapLock sync.Mutex
var OpCallCodeResMap = make(map[string]OpCallCodeRes)

type OpCallCodeRes struct {
	CodeHash common.Hash
	Code     []byte
}

var ExeTxLock OwnedMutex = *NewOwnedMutex()

// OwnedMutex 是一个带有所有权概念的锁
type OwnedMutex struct {
	mu    sync.Mutex
	cond  *sync.Cond
	owner string
}

// NewOwnedMutex 创建一个新的 OwnedMutex 实例
func NewOwnedMutex() *OwnedMutex {
	m := &OwnedMutex{}
	m.cond = sync.NewCond(&m.mu)
	return m
}

var GLobalLock sync.Mutex
var GlobalLockMap = make(map[string]bool)
