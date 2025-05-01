package message

import (
	"blockEmulator/core"
	"math/big"
)

var (
	CClientRequestAccountState MessageType = "RequestAccountState"
	CClientRequestTransaction  MessageType = "RequestAccountTransactions"
)
var (
	CAccountStateForClient       MessageType = "AccountStateForClient"
	CAccountTransactionForClient MessageType = "AccountTransactionsForClient"
)

var (
	DAccountBalanceQuery MessageType = "DAccBalQ"
	DAccountBalanceReply MessageType = "DAccBalR"
)

type ClientRequestAccountState struct {
	AccountAddr string
	ClientIp    string
}
type AccountStateForClient struct {
	AccountAddr   string
	ShardID       int
	BlockHeight   int
	BlockHash     []byte
	StateTrieRoot []byte
	AccountState  *core.AccountState
}

type ClientRequestTransaction struct {
	AccountAddr string
	ClientIp    string
}

type AccountTransactionsForClient struct {
	AccountAddr string
	ShardID     int
	Txs         []*core.Transaction
}

type AccountBalanceQuery struct {
	Addr   string
	TXType string
}

type AccountBalanceReply struct {
	Addr    string
	Balance *big.Int
	TXType  string
}
