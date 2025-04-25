package message

import "blockEmulator/core"

var (
	CClientRequestAccountState MessageType = "RequestAccountState"
	CClientRequestTransaction  MessageType = "RequestAccountTransactions"
)
var (
	CAccountStateForClient       MessageType = "AccountStateForClient"
	CAccountTransactionForClient MessageType = "AccountTransactionsForClient"
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
