package global

import (
	"github.com/ethereum/go-ethereum/common"
	"sync"
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

//
//type OpCallRes struct {
//	Journal []state.Wrapper
//	Ret []byte
//	Err string
//	Gas uint64
//}
//
//var OpCallResMap = make(map[string]OpCallRes)
//var OpCallResMapLock sync.Mutex
//
//
//



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

// LockWithOwner 使用指定的所有者字符串获取锁
func (m *OwnedMutex) LockWithOwner(owner string) {
	//m.mu.Lock()
	//defer m.mu.Unlock()
	//
	//for m.owner != "" && m.owner != owner {
	//	m.cond.Wait()
	//}
	//
	//m.owner = owner
}

// Unlock 释放锁
func (m *OwnedMutex) Unlock() {
	//m.mu.Lock()
	//defer m.mu.Unlock()
	//
	//m.owner = ""
	//m.cond.Signal()
}


var GLobalLock sync.Mutex
var GlobalLockMap = make(map[string]bool)