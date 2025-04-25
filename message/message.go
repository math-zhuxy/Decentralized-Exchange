package message

import (
	"blockEmulator/core"
	"blockEmulator/shard"
	"blockEmulator/vm/state"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"time"
)

var prefixMSGtypeLen = 30

type MessageType string
type RequestType string

const (
	CPrePrepare        MessageType = "preprepare"
	CPrepare           MessageType = "prepare"
	CCommit            MessageType = "commit"
	CRequestOldrequest MessageType = "requestOldrequest"
	CSendOldrequest    MessageType = "sendOldrequest"
	CStop              MessageType = "stop"

	CRelay      MessageType = "relay"
	CInject     MessageType = "inject"
	CInjectHead MessageType = "injectHead"

	CBlockInfo              MessageType = "BlockInfo"
	CSeqIDinfo              MessageType = "SequenceID"
	CContractCreateSuccess  MessageType = "ContractCreate"
	CContractExecuteSuccess MessageType = "ContractExecute"

	CQueryNodeAccount    MessageType = "QueryNodeAcc"
	CResponseNodeAccount MessageType = "ResNodeAcc"

	CQueryAccount    MessageType = "QueryAccount"
	CResponseAccount MessageType = "ResAccount"

	CNodeInfo MessageType = "NodeInfo"
	CTxInfo   MessageType = "TxInfo"

	COpbalanceReq MessageType = "OpBalanceReq"
	COpbalanceRes MessageType = "OpBalanceRes"

	COpDelegateCallReq MessageType = "OpDelCallReq"
	COpDelegateCallRes MessageType = "OpDelCallRes"

	COpCallCodeReq MessageType = "OpCallcodeReq"
	COpCallCodeRes MessageType = "OpCallcodeRes"

	COpCallReq MessageType = "OpCallReq"
	COpCallRes MessageType = "OpCallRes"
	COpStaticCallReq MessageType = "OpStaticCallReq"
	COpStaticCallRes MessageType = "OpStaticCallRes"

	COpExeLogOrRollback MessageType = "OpExeLogOrRollback"


	COpCreateReq MessageType = "OpCreateReq"
	COpCreateRes MessageType = "OpCreateRes"


	CQueryContractResultReq MessageType = "QueryConResReq"
	CQueryContractResultRes MessageType = "QueryConResRes"
)

var (
	BlockRequest RequestType = "Block"
	// add more types
	// ...

)

type QueryContractResultReq struct {
	UUID string
	FromShardID uint64
	FromNodeID uint64
}

type QueryContractResultRes struct {
	Success bool
	UUID string
	Result []byte
}

type OpCreateReq struct {
	Caller []byte
	Addr common.Address
	Code []byte
	//Hash common.Hash
	Value uint64
	Gas uint64

	FromShardId uint64
	FromNodeId uint64
	Id string
	UUID string
}

type OpCreateRes struct {
	Addr common.Address
	Ret []byte
	Err string
	Id string
	Journal []state.Wrapper

}

//type ExeLogOrRollback struct {
//	Log []state.Wrapper
//	Rollback bool
//}

type OpCallReq struct {
	Caller []byte
	Addr common.Address
	Value uint64
	Gas uint64
	Input []byte

	FromShardId uint64
	FromNodeId uint64
	Id string
	UUID string
}

type OpStaticCallReq struct {
	Caller []byte
	Addr common.Address

	Gas uint64
	Input []byte

	FromShardId uint64
	FromNodeId uint64
	Id string
	UUID string
}

type OpCallRes struct {
	Journal []state.Wrapper
	Ret []byte
	Err string
	Id string
	UUID string
}

type OpStaticCallRes struct {
	//Journal []state.Wrapper
	Ret []byte
	Err string
	Id string
	UUID string
}

type OpCallCodeReq struct {
	Address     string
	FromShardId uint64
	FromNodeId  uint64
}

type OpCallCodeRes struct {
	Address  string
	CodeHash common.Hash
	Code     []byte
}

type OpDelegateCallReq struct {
	Address     string
	FromShardId uint64
	FromNodeId  uint64
}

type OpDelegateCallRes struct {
	Address  string
	CodeHash common.Hash
	Code     []byte
}

type OpBalanceReq struct {
	Address     string
	FromShardId uint64
	FromNodeId  uint64
}

type OpBalanceRes struct {
	Address string
	Balance uint64
}

type RawMessage struct {
	Content []byte // the content of raw message, txs and blocks (most cases) included
}

type Request struct {
	RequestType RequestType
	Msg         RawMessage // request message
	ReqTime     time.Time  // request time
}

type PrePrepare struct {
	RequestMsg *Request // the request message should be pre-prepared
	Digest     []byte   // the digest of this request, which is the only identifier
	SeqID      uint64
}

type Prepare struct {
	Digest     []byte // To identify which request is prepared by this node
	SeqID      uint64
	SenderNode *shard.Node // To identify who send this message
}

type Commit struct {
	Digest     []byte // To identify which request is prepared by this node
	SeqID      uint64
	SenderNode *shard.Node // To identify who send this message
}

type Reply struct {
	MessageID  uint64
	SenderNode *shard.Node
	Result     bool
}

type RequestOldMessage struct {
	SeqStartHeight uint64
	SeqEndHeight   uint64
	ServerNode     *shard.Node // send this request to the server node
	SenderNode     *shard.Node
}

type SendOldMessage struct {
	SeqStartHeight uint64
	SeqEndHeight   uint64
	OldRequest     []*Request
	SenderNode     *shard.Node
}

type InjectTxs struct {
	Txs       []*core.Transaction
	ToShardID uint64
}

type BlockInfoMsg struct {
	BlockBodyLength int
	ExcutedTxs      []*core.Transaction // txs which are excuted completely
	Epoch           int

	ProposeTime   time.Time // record the propose time of this block (txs)
	CommitTime    time.Time // record the commit time of this block (txs)
	SenderShardID uint64

	// for transaction relay
	Relay1TxNum uint64              // the number of cross shard txs
	Relay1Txs   []*core.Transaction // cross transactions in chain first time

	// for broker
	Broker1TxNum uint64              // the number of broker 1
	Broker1Txs   []*core.Transaction // cross transactions at first time by broker
	Broker2TxNum uint64              // the number of broker 2
	Broker2Txs   []*core.Transaction // cross transactions at second time by broker
	AllocatedTxs []*core.Transaction // allocated the Txs for  broker
}

type ContractCreateSuccess struct {
	ShardId uint64
	Addr    string
}

type ContractExecuteSuccess struct {
	ShardId uint64
	Addr    string
}

type SeqIDinfo struct {
	SenderShardID uint64
	SenderSeq     uint64
}

type QueryAccount struct {
	FromShardID uint64
	FromNodeID  uint64
	Account     string
}

type AccountRes struct {
	Account string
	Balance uint64
	Code []byte
}

type QueryNodeAccount struct {
	FromNodeID uint64
}

type NodeAccount struct {
	Account string
	NodeId  uint64
}

type NodeInfo struct {
	NodeId        uint64
	ShardId       uint64
	NodeIP        string
	NodePublicKey string

	NodeAccount string
	NodeBalance uint64
}

type TxInfo struct {
	TxHash        []byte
	IsContract    bool
	GasUsed       uint64
	GasPrice      uint64
	GasLimit      uint64
	IsSuccess     bool
	ExecuteResult []byte
	ExecuteTime   time.Time


	From common.Address
	To common.Address
	Input []byte
	Value *big.Int
	UUID string
	Res []byte
}

func MergeMessage(msgType MessageType, content []byte) []byte {
	b := make([]byte, prefixMSGtypeLen)
	for i, v := range []byte(msgType) {
		b[i] = v
	}
	merge := append(b, content...)
	return merge
}

func SplitMessage(message []byte) (MessageType, []byte) {
	msgTypeBytes := message[:prefixMSGtypeLen]
	msgType_pruned := make([]byte, 0)
	for _, v := range msgTypeBytes {
		if v != byte(0) {
			msgType_pruned = append(msgType_pruned, v)
		}
	}
	msgType := string(msgType_pruned)
	content := message[prefixMSGtypeLen:]
	return MessageType(msgType), content
}
